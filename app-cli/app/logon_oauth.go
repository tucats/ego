package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
)

const (
	oauthCallbackPath    = "/callback"
	oauthCallbackTimeout = 2 * time.Minute
	oauthDefaultClientID = "ego-cli"
	oauthDefaultScopes   = "openid profile ego:read ego:write ego:admin ego:code"
)

// oauthDiscovery holds the subset of the OIDC discovery document that the
// CLI needs to drive the Authorization Code + PKCE flow.
type oauthDiscovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
}

// oauthTokenResponse is the JSON body returned by an OAuth2 token endpoint
// for both authorization_code and refresh_token grant types.
type oauthTokenResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int64  `json:"expires_in"`
	RefreshToken     string `json:"refresh_token,omitempty"`
	Scope            string `json:"scope,omitempty"`
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// logonOAuth drives an OAuth2 Authorization Code + PKCE login from the CLI.
// It first attempts a silent refresh using any stored refresh token; if that
// fails (or no refresh token exists) it opens the system browser to the
// authorization URL and waits for the loopback callback.
func logonOAuth(c *cli.Context) error {
	// Set the API server URL from --logon-server (or existing config). This
	// updates LogonServerSetting for subsequent non-logon commands, exactly as
	// the password flow does.
	_, err := findLogonServer(c)
	if err != nil {
		return errors.New(err)
	}

	// Resolve the OAuth2 issuer URL independently of the API server.
	issuer, err := determineIssuer(c)
	if err != nil {
		return errors.New(err)
	}

	// --- Silent refresh path ---
	// If a refresh token is stored, attempt to use it before touching a browser.
	if refreshToken := settings.Get(defs.LogonRefreshTokenSetting); refreshToken != "" {
		ui.Log(ui.RestLogger, "logon.oauth.refresh", ui.A{})

		disc, discErr := fetchOIDCDiscovery(issuer)
		if discErr == nil {
			tok, refreshErr := refreshAccessToken(disc.TokenEndpoint, refreshToken, oauthClientID())
			if refreshErr == nil {
				return storeOAuthTokens(tok, extractSubject(tok.AccessToken))
			}

			ui.Log(ui.RestLogger, "logon.oauth.refresh.failed", ui.A{"error": refreshErr})
		}

		// Clear the stale refresh token so we don't attempt it again.
		settings.Set(defs.LogonRefreshTokenSetting, "")
	}

	// --- Browser authorization flow ---
	disc, err := fetchOIDCDiscovery(issuer)
	if err != nil {
		return errors.New(err)
	}

	verifier, challenge, state, err := generatePKCE()
	if err != nil {
		return errors.New(err)
	}

	// Bind the loopback listener before building the redirect URI so the port is known.
	port, codeCh, stopServer, err := startCallbackServer(state)
	if err != nil {
		return errors.New(err)
	}

	defer stopServer()

	redirectURI := fmt.Sprintf("http://localhost:%d%s", port, oauthCallbackPath)
	clientID := oauthClientID()
	scopes := oauthScopes()

	// Build the authorization URL.
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", scopes)
	params.Set("state", state)
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", "S256")

	authURL := disc.AuthorizationEndpoint
	if strings.Contains(authURL, "?") {
		authURL += "&" + params.Encode()
	} else {
		authURL += "?" + params.Encode()
	}

	// Best-effort browser open. If it fails, print the URL to console output so the
	// user can just open it manually. Note that the pattern here is that browser open
	// does not work, we always print the URL and timeout warning even if quiet mode is
	// enabled. If browser open works, then we print the waiting message without the URL,
	// and in this case will also suppress the timeout message if quiet mode is enabled.
	err = openBrowser(authURL)
	if err != nil {
		ui.SayAlways("logon.oauth.browser.url", ui.A{"url": authURL})
		ui.SayAlways("logon.oauth.waiting", ui.A{})
	} else {
		ui.Say("logon.oauth.waiting", ui.A{})
	}

	// Wait for the callback code or timeout.
	var authCode string

	select {
	case result := <-codeCh:
		authCode = result
	case <-time.After(oauthCallbackTimeout):
		ui.Say("logon.oauth.timeout", ui.A{})

		return errors.New(fmt.Errorf("%s", i18n.T("logon.oauth.timeout")))
	}

	// Exchange the authorization code for tokens.
	tok, err := exchangeCode(disc.TokenEndpoint, authCode, verifier, clientID, redirectURI)
	if err != nil {
		return errors.New(err)
	}

	subject := extractSubject(tok.AccessToken)

	return storeOAuthTokens(tok, subject)
}

// determineIssuer resolves the OAuth2 Authorization Server issuer URL using
// the following priority:
//
//  1. --oauth-server CLI flag
//  2. ego.logon.oauth.server config setting
//  3. ego.server.oauth.as.issuer  (Ego's own AS)
//  4. ego.server.oauth.provider   (external IdP configured for RS mode)
func determineIssuer(c *cli.Context) (string, error) {
	if issuer, found := c.String("oauth-server"); found && issuer != "" {
		return strings.TrimSuffix(issuer, "/"), nil
	}

	if issuer := settings.Get(defs.OAuthCLIServerSetting); issuer != "" {
		return strings.TrimSuffix(issuer, "/"), nil
	}

	if issuer := settings.Get(defs.OAuthASIssuerSetting); issuer != "" {
		return strings.TrimSuffix(issuer, "/"), nil
	}

	if issuer := settings.Get(defs.OAuthProviderSetting); issuer != "" {
		return strings.TrimSuffix(issuer, "/"), nil
	}

	return "", fmt.Errorf("%s", i18n.T("logon.oauth.no.issuer"))
}

// fetchOIDCDiscovery retrieves the OIDC discovery document from
// {issuer}/.well-known/openid-configuration and extracts the endpoints
// the CLI needs.
func fetchOIDCDiscovery(issuer string) (*oauthDiscovery, error) {
	discoveryURL := strings.TrimSuffix(issuer, "/") + "/.well-known/openid-configuration"

	resp, err := http.Get(discoveryURL) //nolint:gosec // URL is from trusted config
	if err != nil {
		return nil, fmt.Errorf("fetching OIDC discovery from %s: %w", discoveryURL, err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading OIDC discovery response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OIDC discovery returned HTTP %d from %s", resp.StatusCode, discoveryURL)
	}

	var doc oauthDiscovery
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parsing OIDC discovery document: %w", err)
	}

	if doc.AuthorizationEndpoint == "" {
		return nil, fmt.Errorf("OIDC discovery document missing authorization_endpoint")
	}

	if doc.TokenEndpoint == "" {
		return nil, fmt.Errorf("OIDC discovery document missing token_endpoint")
	}

	return &doc, nil
}

// generatePKCE creates a cryptographically random PKCE verifier, its S256
// challenge, and a random state token. All three are base64url-encoded.
func generatePKCE() (verifier, challenge, state string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", "", "", fmt.Errorf("generating PKCE verifier: %w", err)
	}

	verifier = base64.RawURLEncoding.EncodeToString(raw)

	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])

	stateBuf := make([]byte, 16)
	if _, err = rand.Read(stateBuf); err != nil {
		return "", "", "", fmt.Errorf("generating state token: %w", err)
	}

	state = base64.RawURLEncoding.EncodeToString(stateBuf)

	return verifier, challenge, state, nil
}

// startCallbackServer binds a local HTTP listener on a random TCP port and
// waits for a single OAuth2 callback request containing the authorization
// code. The assigned port, a receive-only channel that delivers the auth code,
// and a stop function are returned.
//
// The channel delivers exactly one value: the authorization code string, or
// an empty string if the callback contained an error. The stop function shuts
// down the listener; it is safe to call multiple times.
func startCallbackServer(expectedState string) (port int, codeCh <-chan string, stop func(), err error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, nil, nil, fmt.Errorf("starting OAuth2 callback listener: %w", err)
	}

	port = ln.Addr().(*net.TCPAddr).Port
	ch := make(chan string, 1)

	srv := &http.Server{}

	mux := http.NewServeMux()
	mux.HandleFunc(oauthCallbackPath, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		if q.Get("state") != expectedState {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			ch <- ""

			return
		}

		if errParam := q.Get("error"); errParam != "" {
			desc := q.Get("error_description")
			http.Error(w, errParam+": "+desc, http.StatusBadRequest)

			ui.Say("logon.oauth.callback.error", ui.A{"error": errParam, "description": desc})
			ch <- ""

			return
		}

		code := q.Get("code")

		fmt.Fprintln(w, i18n.T("logon.oauth.browser.done"))
		ch <- code

		go func() {
			// Shut down after a short delay so the response is fully written.
			time.Sleep(100 * time.Millisecond)

			_ = srv.Shutdown(context.Background())
		}()
	})

	srv.Handler = mux

	go func() { _ = srv.Serve(ln) }()

	stop = func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_ = srv.Shutdown(ctx)
	}

	return port, ch, stop, nil
}

// openBrowser opens the system default browser to the given URL.
// Errors are non-fatal — the URL is always printed to the console
// before this is called, so the user can open it manually.
func openBrowser(rawURL string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}

	return cmd.Start()
}

// exchangeCode sends the authorization code to the token endpoint and returns
// the resulting token response. The PKCE code_verifier and the exact redirect
// URI used in the authorization request must be included.
func exchangeCode(tokenEndpoint, code, verifier, clientID, redirectURI string) (*oauthTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", clientID)
	form.Set("code_verifier", verifier)

	return postTokenRequest(tokenEndpoint, form)
}

// refreshAccessToken uses the stored refresh token to silently obtain a new
// access token without requiring browser interaction.
func refreshAccessToken(tokenEndpoint, refreshToken, clientID string) (*oauthTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", clientID)

	return postTokenRequest(tokenEndpoint, form)
}

// postTokenRequest sends a POST with form-encoded parameters to the token
// endpoint and decodes the JSON response.
func postTokenRequest(tokenEndpoint string, form url.Values) (*oauthTokenResponse, error) {
	req, err := http.NewRequest(http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("building token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token endpoint POST to %s: %w", tokenEndpoint, err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading token response: %w", err)
	}

	var tok oauthTokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("parsing token response: %w", err)
	}

	if tok.Error != "" {
		return nil, fmt.Errorf("token endpoint error %q: %s", tok.Error, tok.ErrorDescription)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned HTTP %d", resp.StatusCode)
	}

	if tok.AccessToken == "" {
		return nil, fmt.Errorf("token response contains no access_token")
	}

	return &tok, nil
}

// storeOAuthTokens persists the access token, its expiration, and any refresh
// token into the active settings profile. Other CLI commands read
// LogonTokenSetting as a Bearer token, so they transparently work with a JWT.
func storeOAuthTokens(tok *oauthTokenResponse, subject string) error {
	settings.Set(defs.LogonTokenSetting, tok.AccessToken)

	// Compute an absolute expiry time from the relative expires_in seconds.
	if tok.ExpiresIn > 0 {
		expiry := time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
		settings.Set(defs.LogonTokenExpirationSetting, expiry.Format(time.RFC3339))
	} else {
		settings.Set(defs.LogonTokenExpirationSetting, "")
	}

	// Store (or clear) the refresh token.
	settings.Set(defs.LogonRefreshTokenSetting, tok.RefreshToken)

	if err := settings.Save(); err != nil {
		return errors.New(err)
	}

	expiry := settings.Get(defs.LogonTokenExpirationSetting)
	msg := i18n.M("logged.in", map[string]any{
		"user":    subject,
		"expires": expiry,
	})

	ui.Say("%s", msg)

	return nil
}

// extractSubject decodes the JWT payload (middle segment) without validating
// the signature and returns the "sub" claim. Returns an empty string if the
// token cannot be decoded — this is display-only, never used for auth.
func extractSubject(accessToken string) string {
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return ""
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}

	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}

	if sub, ok := claims["sub"].(string); ok {
		return sub
	}

	return ""
}

// oauthClientID returns the configured CLI client_id, defaulting to "ego-cli".
func oauthClientID() string {
	if id := settings.Get(defs.OAuthCLIClientIDSetting); id != "" {
		return id
	}

	return oauthDefaultClientID
}

// oauthScopes returns the configured scope string, ensuring "openid" is always
// present.
func oauthScopes() string {
	scopes := settings.Get(defs.OAuthCLIScopesSetting)
	if scopes == "" {
		scopes = oauthDefaultScopes
	}

	if !strings.Contains(scopes, "openid") {
		scopes = "openid " + scopes
	}

	return scopes
}
