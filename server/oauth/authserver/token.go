package authserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/router"
	"github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/util"
)

// asGlobalConfig holds the resolved configuration loaded at startup.
// It is accessible to all handlers in the package without being re-read on
// every request.
var asGlobalConfig asConfig

// TokenHandler processes POST /oauth2/token requests.
//
// It dispatches to the appropriate sub-handler based on the "grant_type" form
// parameter:
//   - "authorization_code" — exchange a code for tokens
//   - "client_credentials" — server-to-server token without user login
//   - "refresh_token"      — obtain a new access token from a refresh token
func TokenHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	if err := r.ParseForm(); err != nil {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.body.parse"), http.StatusBadRequest)
	}

	grantType := r.FormValue("grant_type")

	switch grantType {
	case "authorization_code":
		return handleAuthorizationCodeGrant(session, w, r)
	case "client_credentials":
		return handleClientCredentialsGrant(session, w, r)
	case "refresh_token":
		return handleRefreshTokenGrant(session, w, r)
	default:
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.invalid.grant"), http.StatusBadRequest)
	}
}

// handleAuthorizationCodeGrant exchanges a short-lived authorization code for
// an access token (and optionally an ID token and refresh token).
//
// Required form fields: grant_type, code, redirect_uri, client_id
// Optional: client_secret (for confidential clients), code_verifier (for PKCE).
func handleAuthorizationCodeGrant(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	clientID, clientSecret := validateBasicAuth(r)
	code := r.FormValue("code")
	redirectURI := r.FormValue("redirect_uri")
	codeVerifier := r.FormValue("code_verifier")

	client := findClient(clientID)
	if client == nil || !validateClientSecret(client, clientSecret) {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.invalid.client"), http.StatusUnauthorized)
	}

	if !clientAllowsGrant(client, "authorization_code") {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.invalid.grant"), http.StatusBadRequest)
	}

	pending, found := consumeCode(code)
	if !found {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.invalid.code"), http.StatusBadRequest)
	}

	// Audit: RFC 6749 §4.1.3 client binding: verify that the client presenting
	// the authorization code is the same client that received it.
	//
	// Why this check is necessary:
	//
	//   An authorization code is issued to a specific client at the moment the
	//   user authenticates (AuthorizePostHandler stores pending.ClientID).
	//   Without this check, a different client could present that code at the
	//   token endpoint and exchange it for tokens — effectively stealing the
	//   user's session.
	//
	//   For public clients the attack is most practical because no client secret
	//   is needed: any caller who knows the client_id (which is public by
	//   definition) and has obtained the code (e.g., by intercepting the redirect
	//   URL) could complete the exchange.  PKCE adds another layer for public
	//   clients, but relying on PKCE alone without binding the code to its
	//   rightful client violates the spec and provides weaker guarantees if the
	//   code is stolen before PKCE verification runs.
	//
	//   For confidential clients the attacker must also know the target client's
	//   secret, which raises the bar — but RFC 6749 still requires the check
	//   and defense-in-depth demands it.
	//
	// Why we return the same 401 as "invalid client credentials" (not a new error):
	//
	//   Returning a different status or body for a client-mismatch vs. a bad
	//   secret would let an attacker enumerate: "is this code issued to client A
	//   or client B?"  Responding identically in all credential-failure cases
	//   denies that information.  The server log records the full detail for the
	//   operator; the client-facing response stays intentionally generic.
	if pending.ClientID != clientID {
		ui.Log(ui.ServerLogger, "oauth.as.authorize.denied", ui.A{
			"client": pending.ClientID,
			"user":   pending.Username,
			"reason": "code presented by a different client (client_id mismatch); H1",
		})

		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.invalid.client"), http.StatusUnauthorized)
	}

	// The redirect_uri in the token request must match the one used in the
	// authorization request (RFC 6749 §4.1.3).
	if pending.RedirectURI != redirectURI {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.invalid.redirect"), http.StatusBadRequest)
	}

	// OAUTH-H3: Public clients — those registered without a client_secret_hash —
	// MUST use PKCE (RFC 9700 §2.1.1, OAuth 2.0 Security Best Current Practice).
	//
	// Without PKCE a stolen authorization code is immediately usable by any
	// party that knows the client_id (which is public by definition for a public
	// client).  PKCE binds the code to the specific device that initiated the
	// authorization flow via the code_verifier / code_challenge pair, so only
	// the original requester can complete the exchange.
	//
	// We check pending.CodeChallenge rather than the current request's
	// code_verifier because the challenge is stored by the AS at authorization
	// time.  An empty challenge means the client omitted PKCE in the first step;
	// we must reject the exchange here regardless of what code_verifier (if any)
	// the client sends now.
	if client.ClientSecretHash == "" && pending.CodeChallenge == "" {
		ui.Log(ui.ServerLogger, "oauth.as.pkce.missing", ui.A{
			"client": clientID,
			"user":   pending.Username,
		})

		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.pkce.required"), http.StatusBadRequest)
	}

	// Verify PKCE if the authorization request included a code_challenge.
	// For confidential clients this check is optional (code_challenge may be
	// empty); for public clients it is mandatory and guaranteed non-empty by
	// the guard above.
	if err := verifyPKCE(pending, codeVerifier); err != nil {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.invalid.pkce"), http.StatusBadRequest)
	}

	scope := strings.Join(pending.Scopes, " ")
	accessToken, _, err := createAccessToken(asGlobalConfig, clientID, pending.Username, clientID, scope)

	if err != nil {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.token.create"), http.StatusInternalServerError)
	}

	resp := tokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(asGlobalConfig.TokenExpiration.Seconds()),
		Scope:       scope,
	}

	// Issue an ID token when the "openid" scope was granted.
	if strings.Contains(scope, "openid") {
		idToken, idErr := createIDToken(asGlobalConfig, clientID, pending.Username)
		if idErr == nil {
			resp.IDToken = idToken
		}
	}

	// Issue a refresh token when the client supports it.
	if clientAllowsGrant(client, "refresh_token") {
		refreshToken, refreshErr := generateRefreshToken(clientID, pending.Username, pending.Scopes)
		if refreshErr == nil {
			resp.RefreshToken = refreshToken
		}
	}

	return writeTokenResponse(w, session, resp)
}

// handleClientCredentialsGrant issues an access token directly to a registered
// client based on its credentials, without any user login.
//
// This grant is used for server-to-server (machine-to-machine) access.
// Required form fields: grant_type, client_id, client_secret, scope.
func handleClientCredentialsGrant(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	clientID, clientSecret := validateBasicAuth(r)
	scopeStr := r.FormValue("scope")

	client := findClient(clientID)
	if client == nil || !validateClientSecret(client, clientSecret) {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.invalid.client"), http.StatusUnauthorized)
	}

	if !clientAllowsGrant(client, "client_credentials") {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.invalid.grant"), http.StatusBadRequest)
	}

	// Validate requested scopes against the client's allowed scopes.
	requested := splitScope(scopeStr)
	if len(requested) > 0 && !clientAllowsScope(client, requested) {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.invalid.scope"), http.StatusBadRequest)
	}

	// If no scope was requested, grant all of the client's allowed scopes.
	if len(requested) == 0 {
		requested = client.Scopes
	}

	scope := strings.Join(requested, " ")

	// For client_credentials the "sub" (subject) is empty — there is no user.
	accessToken, _, err := createAccessToken(asGlobalConfig, clientID, "", clientID, scope)
	if err != nil {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.token.create"), http.StatusInternalServerError)
	}

	ui.Log(ui.ServerLogger, "oauth.as.client.credentials.issued", ui.A{
		"client": clientID,
		"scope":  scope,
	})

	return writeTokenResponse(w, session, tokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(asGlobalConfig.TokenExpiration.Seconds()),
		Scope:       scope,
	})
}

// handleRefreshTokenGrant exchanges a refresh token for a new access token.
// The old refresh token is consumed and a new one is issued (token rotation).
//
// Required form fields: grant_type, refresh_token, client_id
// Optional: client_secret (for confidential clients), scope (may narrow).
func handleRefreshTokenGrant(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	clientID, clientSecret := validateBasicAuth(r)
	refreshTokenStr := r.FormValue("refresh_token")

	client := findClient(clientID)
	if client == nil || !validateClientSecret(client, clientSecret) {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.invalid.client"), http.StatusUnauthorized)
	}

	if !clientAllowsGrant(client, "refresh_token") {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.invalid.grant"), http.StatusBadRequest)
	}

	data, found := consumeRefreshToken(refreshTokenStr)
	if !found {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.invalid.refresh"), http.StatusBadRequest)
	}

	// The refresh token must belong to the client presenting it.
	if data.ClientID != clientID {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.invalid.client"), http.StatusUnauthorized)
	}

	scope := strings.Join(data.Scopes, " ")

	accessToken, jti, err := createAccessToken(asGlobalConfig, clientID, data.Username, clientID, scope)
	if err != nil {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.token.create"), http.StatusInternalServerError)
	}

	ui.Log(ui.ServerLogger, "oauth.as.token.refreshed", ui.A{
		"client": clientID,
		"jti":    jti,
	})

	// Issue a new refresh token (rotation — the old one was deleted by consumeRefreshToken).
	newRefreshToken, refreshErr := generateRefreshToken(clientID, data.Username, data.Scopes)

	resp := tokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(asGlobalConfig.TokenExpiration.Seconds()),
		Scope:       scope,
	}

	if refreshErr == nil {
		resp.RefreshToken = newRefreshToken
	}

	// Re-issue an ID token when openid is in scope.
	if strings.Contains(scope, "openid") && data.Username != "" {
		idToken, idErr := createIDToken(asGlobalConfig, clientID, data.Username)
		if idErr == nil {
			resp.IDToken = idToken
		}
	}

	return writeTokenResponse(w, session, resp)
}

// writeTokenResponse serializes a tokenResponse as JSON and writes it to w.
// Returns the HTTP status code (200 on success).
func writeTokenResponse(w http.ResponseWriter, session *router.Session, resp tokenResponse) int {
	b, err := json.Marshal(resp)
	if err != nil {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.token.serialize"), http.StatusInternalServerError)
	}

	w.Header().Set(defs.ContentTypeHeader, defs.JSONMediaType)
	// RFC 6749 requires Cache-Control: no-store on token responses to prevent
	// browsers and proxies from caching tokens.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)

	return http.StatusOK
}

// validateBasicAuth extracts and validates HTTP Basic Auth credentials from a
// token endpoint request.  Basic Auth is the preferred mechanism for confidential
// clients (RFC 6749 §2.3.1).  Returns the clientID and clientSecret, or empty
// strings if no Basic Auth header was present.
func validateBasicAuth(r *http.Request) (clientID, clientSecret string) {
	cid, csec, ok := r.BasicAuth()
	if ok {
		return cid, csec
	}

	// Fall back to form-encoded credentials if no Basic Auth header is present.
	return r.FormValue("client_id"), r.FormValue("client_secret")
}

// validatePassword wraps auth.ValidatePassword, short-circuiting immediately
// for blank credentials to avoid an unnecessary database lookup.
func validatePassword(sessionID int, username, password string) bool {
	if username == "" || password == "" {
		return false
	}

	return auth.ValidatePassword(sessionID, username, password)
}
