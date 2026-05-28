package oauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// makeFullOIDCServer creates a test HTTP server that handles both the OIDC
// discovery endpoint and (optionally) the token endpoint.  The tokenHandler
// parameter may be nil to skip the token endpoint.
func makeFullOIDCServer(t *testing.T, tokenHandler http.HandlerFunc) *httptest.Server {
	t.Helper()

	var srv *httptest.Server

	mux := http.NewServeMux()

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		doc := validDiscoveryDoc("http://" + r.Host)
		body, _ := json.Marshal(doc)
		
		w.Header().Set("Content-Type", "application/json")
		w.Write(body) //nolint:errcheck
	})

	if tokenHandler != nil {
		mux.Handle("/oauth2/token", tokenHandler)
	}

	srv = httptest.NewServer(mux)

	return srv
}

// TestAuthorizeURLContainsRequiredParams verifies that AuthorizeURL returns a URL
// that contains all required OAuth2/PKCE query parameters.
func TestAuthorizeURLContainsRequiredParams(t *testing.T) {
	resetDiscoveryCache()

	srv := makeFullOIDCServer(t, nil)
	defer srv.Close()

	resetDiscoveryCache()

	cfg := rsConfig{
		Provider:    srv.URL,
		ClientID:    "test-client",
		Scopes:      "openid profile",
		RedirectURI: "https://example.com/callback",
	}

	redirectURL, state, verifier, err := AuthorizeURL(cfg)
	if err != nil {
		t.Fatalf("AuthorizeURL() error: %v", err)
	}

	// Parse the returned URL so we can inspect its query parameters.
	parsed, err := url.Parse(redirectURL)
	if err != nil {
		t.Fatalf("parsing redirect URL %q: %v", redirectURL, err)
	}

	q := parsed.Query()

	// Required OAuth2 parameters.
	checks := map[string]string{
		"response_type":         "code",
		"client_id":             "test-client",
		"redirect_uri":          "https://example.com/callback",
		"scope":                 "openid profile",
		"state":                 state,
		"code_challenge_method": "S256",
	}

	for param, want := range checks {
		got := q.Get(param)
		if got != want {
			t.Errorf("query param %q = %q, want %q", param, got, want)
		}
	}

	// code_challenge must equal BASE64URL(SHA256(verifier)).
	expectedChallenge := codeChallenge(verifier)
	if q.Get("code_challenge") != expectedChallenge {
		t.Errorf("code_challenge = %q, want %q", q.Get("code_challenge"), expectedChallenge)
	}

	// The URL should point to the authorization endpoint declared in the discovery doc.
	if !strings.HasPrefix(redirectURL, srv.URL+"/oauth2/authorize") {
		t.Errorf("redirect URL %q should start with the authorization_endpoint", redirectURL)
	}
}

// TestAuthorizeURLStateStored verifies that the state returned by AuthorizeURL is
// stored in the state store and can be validated.
func TestAuthorizeURLStateStored(t *testing.T) {
	resetDiscoveryCache()

	srv := makeFullOIDCServer(t, nil)
	defer srv.Close()

	resetDiscoveryCache()

	cfg := rsConfig{
		Provider:    srv.URL,
		ClientID:    "client",
		Scopes:      "openid",
		RedirectURI: "https://app.example.com/cb",
	}

	_, state, verifier, err := AuthorizeURL(cfg)
	if err != nil {
		t.Fatalf("AuthorizeURL() error: %v", err)
	}

	// validateState should succeed and return our verifier.
	ps, err := validateState(state)
	if err != nil {
		t.Fatalf("validateState() error: %v", err)
	}

	if ps.CodeVerifier != verifier {
		t.Errorf("stored code_verifier = %q, want %q", ps.CodeVerifier, verifier)
	}
}

// TestAuthorizeURLNoDiscovery verifies that AuthorizeURL fails gracefully when
// the IdP is unreachable.
func TestAuthorizeURLNoDiscovery(t *testing.T) {
	resetDiscoveryCache()

	cfg := rsConfig{
		Provider:    "http://127.0.0.1:1",
		ClientID:    "client",
		Scopes:      "openid",
		RedirectURI: "https://app.example.com/cb",
	}

	_, _, _, err := AuthorizeURL(cfg)
	if err == nil {
		t.Error("AuthorizeURL() should fail when the IdP is unreachable")
	}
}

// TestExchangeCodeSuccess verifies that ExchangeCode correctly parses an access
// token from a token endpoint response.
func TestExchangeCodeSuccess(t *testing.T) {
	resetDiscoveryCache()

	tokenResp := map[string]any{
		"access_token": "eyJhbGciOiJFUzI1NiJ9.payload.sig",
		"token_type":   "Bearer",
		"expires_in":   3600,
		"id_token":     "eyJhbGciOiJFUzI1NiJ9.id.sig",
	}

	tokenBody, _ := json.Marshal(tokenResp)

	srv := makeFullOIDCServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(tokenBody) //nolint:errcheck
	}))
	defer srv.Close()

	resetDiscoveryCache()

	cfg := rsConfig{
		Provider:     srv.URL,
		ClientID:     "client",
		ClientSecret: "secret",
		RedirectURI:  "https://app.example.com/cb",
	}

	accessToken, idToken, err := ExchangeCode(cfg, "code123", "verifier456")
	if err != nil {
		t.Fatalf("ExchangeCode() error: %v", err)
	}

	if accessToken != "eyJhbGciOiJFUzI1NiJ9.payload.sig" {
		t.Errorf("access_token = %q, want the mock value", accessToken)
	}

	if idToken != "eyJhbGciOiJFUzI1NiJ9.id.sig" {
		t.Errorf("id_token = %q, want the mock value", idToken)
	}
}

// TestExchangeCodeIdPError verifies that an error response from the IdP token
// endpoint is returned as an error by ExchangeCode.
func TestExchangeCodeIdPError(t *testing.T) {
	resetDiscoveryCache()

	errorResp := map[string]any{
		"error":             "invalid_grant",
		"error_description": "The authorization code has expired.",
	}

	errorBody, _ := json.Marshal(errorResp)

	srv := makeFullOIDCServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write(errorBody) //nolint:errcheck
	}))
	defer srv.Close()

	resetDiscoveryCache()

	cfg := rsConfig{
		Provider:    srv.URL,
		ClientID:    "client",
		RedirectURI: "https://app.example.com/cb",
	}

	_, _, err := ExchangeCode(cfg, "expired-code", "verifier")
	if err == nil {
		t.Fatal("ExchangeCode() should fail when the IdP returns an error")
	}

	if !strings.Contains(err.Error(), "invalid_grant") {
		t.Errorf("error should mention the IdP error code, got: %v", err)
	}
}

// TestExchangeCodeMissingAccessToken verifies that a 200 response with no
// access_token field returns an error.
func TestExchangeCodeMissingAccessToken(t *testing.T) {
	resetDiscoveryCache()

	resp := map[string]any{
		"token_type": "Bearer",
	}

	body, _ := json.Marshal(resp)

	srv := makeFullOIDCServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body) //nolint:errcheck
	}))
	defer srv.Close()

	resetDiscoveryCache()

	cfg := rsConfig{
		Provider:    srv.URL,
		ClientID:    "client",
		RedirectURI: "https://app.example.com/cb",
	}

	_, _, err := ExchangeCode(cfg, "code123", "verifier456")
	if err == nil {
		t.Fatal("ExchangeCode() should fail when access_token is missing")
	}
}
