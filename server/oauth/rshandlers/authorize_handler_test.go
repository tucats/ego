// Package rshandlers_test covers the OAUTH-H5 security fix for
// AuthorizeRedirectHandler: the handler must always use the server-configured
// redirect URI and must not allow callers to substitute an arbitrary one via a
// query parameter.
package rshandlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/router"
	"github.com/tucats/ego/server/oauth/rshandlers"
)

// oidcSrv is a single shared mock OIDC server used by all tests in this file.
//
// Why one shared server instead of a new server per test?
//
// The oauth package's discovery cache stores exactly one document and does not
// key it by provider URL.  If each test created its own httptest.Server at a
// different port, the second test would find a stale discovery document — one
// whose authorization_endpoint points at the first test's (now-closed) server —
// and the redirect URL check would fail with a port mismatch.
//
// Using a single server ensures the discovery cache always holds a valid entry
// for oidcSrv.URL, so all tests that configure the same provider URL get
// consistent, correct behavior.
var oidcSrv *httptest.Server

// TestMain starts the shared mock OIDC server, runs all tests, then shuts the
// server down.  Any test that needs a live provider should call
// configureRS(t, oidcSrv.URL, ...) to point the RS configuration at it.
func TestMain(m *testing.M) {
	oidcSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only serve the OIDC discovery endpoint; all other paths return 404.
		if !strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration") {
			http.NotFound(w, r)

			return
		}

		base := "http://" + r.Host

		// Minimal discovery document — only the fields that Ego actually reads
		// when building an authorization URL.
		doc := map[string]any{
			"issuer":                 base,
			"authorization_endpoint": base + "/oauth2/auth",
			"token_endpoint":         base + "/oauth2/token",
			"jwks_uri":               base + "/.well-known/jwks.json",
		}

		body, _ := json.Marshal(doc)

		w.Header().Set("Content-Type", "application/json")

		w.Write(body) //nolint:errcheck
	}))

	code := m.Run()

	oidcSrv.Close()
	os.Exit(code)
}

// configureRS sets the OAuth2 RS settings to point to the given provider URL
// and configures the given redirect URI.  It registers a t.Cleanup hook that
// clears all settings when the test ends so that one test's settings do not
// carry over into the next.
//
// All tests that need a live IdP should call configureRS(t, oidcSrv.URL, ...).
func configureRS(t *testing.T, providerURL, redirectURI string) {
	t.Helper()

	settings.Set(defs.OAuthProviderSetting, providerURL)
	settings.Set(defs.OAuthRedirectURISetting, redirectURI)
	settings.Set(defs.OAuthClientIDSetting, "test-client")
	settings.Set(defs.OAuthScopesSetting, "openid")

	t.Cleanup(func() {
		settings.Set(defs.OAuthProviderSetting, "")
		settings.Set(defs.OAuthRedirectURISetting, "")
		settings.Set(defs.OAuthClientIDSetting, "")
		settings.Set(defs.OAuthScopesSetting, "")
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// OAUTH-H5: the ?redirect= query parameter must have no effect
// ─────────────────────────────────────────────────────────────────────────────

// TestAuthorizeRedirectIgnoresRedirectParam is the primary regression test for
// OAUTH-H5.  It verifies that passing a ?redirect= query parameter to the
// authorize endpoint does NOT change the redirect_uri embedded in the IdP
// authorization URL.
//
// Before the fix, the handler replaced cfg.RedirectURI with the caller-supplied
// value without any validation, allowing an attacker to redirect users — and
// their authorization codes — to an arbitrary URI.  After the fix the parameter
// is silently ignored and the server-configured URI is always used.
func TestAuthorizeRedirectIgnoresRedirectParam(t *testing.T) {
	const configuredURI = "https://configured.example.com/callback"
	const attackerURI = "https://evil.example.com/steal"

	configureRS(t, oidcSrv.URL, configuredURI)

	// Build a request that includes the ?redirect= parameter an attacker would
	// use to try to hijack the authorization flow.
	req := httptest.NewRequest(
		http.MethodGet,
		"/services/admin/oauth/authorize?redirect="+url.QueryEscape(attackerURI),
		nil,
	)

	w := httptest.NewRecorder()

	status := rshandlers.AuthorizeRedirectHandler(&router.Session{ID: 1}, w, req)

	// The handler must redirect the browser (302 Found).
	if status != http.StatusFound {
		t.Fatalf("status = %d, want %d (Found)", status, http.StatusFound)
	}

	location := w.Header().Get("Location")
	if location == "" {
		t.Fatal("Location header is missing from redirect response")
	}

	// Parse the authorization URL so we can inspect its query parameters.
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parsing Location %q: %v", location, err)
	}

	actualRedirectURI := parsed.Query().Get("redirect_uri")

	// The redirect_uri in the authorization URL must be the server-configured
	// value, not the attacker-supplied one.
	if actualRedirectURI != configuredURI {
		t.Errorf("redirect_uri in authorization URL = %q, want the configured %q",
			actualRedirectURI, configuredURI)
	}

	if strings.Contains(location, "evil.example.com") {
		t.Errorf("Location %q contains the attacker URI — the ?redirect= override was not removed",
			location)
	}
}

// TestAuthorizeRedirectUsesConfiguredURI verifies the happy path: when no
// ?redirect= parameter is present, the authorization URL contains the
// server-configured redirect URI and all required PKCE parameters.
func TestAuthorizeRedirectUsesConfiguredURI(t *testing.T) {
	const configuredURI = "https://myapp.example.com/oauth/callback"

	configureRS(t, oidcSrv.URL, configuredURI)

	req := httptest.NewRequest(http.MethodGet, "/services/admin/oauth/authorize", nil)
	w := httptest.NewRecorder()

	status := rshandlers.AuthorizeRedirectHandler(&router.Session{ID: 2}, w, req)

	if status != http.StatusFound {
		t.Fatalf("status = %d, want %d (Found)", status, http.StatusFound)
	}

	location := w.Header().Get("Location")
	if location == "" {
		t.Fatal("Location header is missing from redirect response")
	}

	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parsing Location %q: %v", location, err)
	}

	q := parsed.Query()

	// All required PKCE / OAuth2 parameters must be present in the
	// authorization URL that Ego sends to the IdP.
	required := []string{
		"response_type", "client_id", "redirect_uri", "scope",
		"state", "code_challenge", "code_challenge_method",
	}

	for _, param := range required {
		if q.Get(param) == "" {
			t.Errorf("authorization URL is missing required parameter %q", param)
		}
	}

	if got := q.Get("redirect_uri"); got != configuredURI {
		t.Errorf("redirect_uri = %q, want %q", got, configuredURI)
	}

	if got := q.Get("code_challenge_method"); got != "S256" {
		t.Errorf("code_challenge_method = %q, want %q", got, "S256")
	}

	// The authorization endpoint must point to the mock server (not some stale
	// cached URL from a prior test).
	if !strings.HasPrefix(location, oidcSrv.URL+"/oauth2/auth") {
		t.Errorf("Location %q does not point to the expected authorization endpoint %q",
			location, oidcSrv.URL+"/oauth2/auth")
	}
}

// TestAuthorizeRedirectNoProvider verifies that the handler returns
// HTTP 503 Service Unavailable when ego.server.oauth.provider is not set.
// A caller must not be able to trigger an IdP flow when no provider is
// configured.
func TestAuthorizeRedirectNoProvider(t *testing.T) {
	settings.Set(defs.OAuthProviderSetting, "")
	settings.Set(defs.OAuthRedirectURISetting, "")

	t.Cleanup(func() {
		settings.Set(defs.OAuthProviderSetting, "")
		settings.Set(defs.OAuthRedirectURISetting, "")
	})

	req := httptest.NewRequest(http.MethodGet, "/services/admin/oauth/authorize", nil)
	w := httptest.NewRecorder()

	status := rshandlers.AuthorizeRedirectHandler(&router.Session{ID: 3}, w, req)

	if status != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d (Service Unavailable)", status, http.StatusServiceUnavailable)
	}
}

// TestAuthorizeRedirectNoRedirectURI verifies that the handler returns
// HTTP 500 when ego.server.oauth.provider is set but
// ego.server.oauth.redirect.uri is empty.  Without a redirect URI, the PKCE
// flow cannot complete and the server must refuse to start it.
func TestAuthorizeRedirectNoRedirectURI(t *testing.T) {
	settings.Set(defs.OAuthProviderSetting, oidcSrv.URL)
	settings.Set(defs.OAuthRedirectURISetting, "") // intentionally empty
	settings.Set(defs.OAuthClientIDSetting, "test-client")
	settings.Set(defs.OAuthScopesSetting, "openid")

	t.Cleanup(func() {
		settings.Set(defs.OAuthProviderSetting, "")
		settings.Set(defs.OAuthRedirectURISetting, "")
		settings.Set(defs.OAuthClientIDSetting, "")
		settings.Set(defs.OAuthScopesSetting, "")
	})

	req := httptest.NewRequest(http.MethodGet, "/services/admin/oauth/authorize", nil)
	w := httptest.NewRecorder()

	status := rshandlers.AuthorizeRedirectHandler(&router.Session{ID: 4}, w, req)

	if status != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d (Internal Server Error)", status, http.StatusInternalServerError)
	}
}

// TestAuthorizeRedirectTwoCallsProduceDifferentStates verifies that each call
// to the handler generates a unique PKCE state token in the authorization URL.
// PKCE security depends on each flow having an unpredictable, single-use state
// value that an attacker cannot guess or replay.
func TestAuthorizeRedirectTwoCallsProduceDifferentStates(t *testing.T) {
	configureRS(t, oidcSrv.URL, "https://app.example.com/cb")

	// First request.
	req1 := httptest.NewRequest(http.MethodGet, "/services/admin/oauth/authorize", nil)
	w1 := httptest.NewRecorder()

	rshandlers.AuthorizeRedirectHandler(&router.Session{ID: 5}, w1, req1) //nolint:errcheck

	// Second request.
	req2 := httptest.NewRequest(http.MethodGet, "/services/admin/oauth/authorize", nil)
	w2 := httptest.NewRecorder()

	rshandlers.AuthorizeRedirectHandler(&router.Session{ID: 6}, w2, req2) //nolint:errcheck

	// Parse the state parameters from both authorization URLs.
	p1, _ := url.Parse(w1.Header().Get("Location"))
	p2, _ := url.Parse(w2.Header().Get("Location"))

	state1 := p1.Query().Get("state")
	state2 := p2.Query().Get("state")

	if state1 == "" || state2 == "" {
		t.Fatal("state parameter is missing from one or both authorization URLs")
	}

	if state1 == state2 {
		t.Error("two consecutive requests produced the same state token — states must be unique per flow")
	}
}
