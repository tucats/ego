package oauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// makeDiscoveryServer starts a test HTTP server that returns the given
// discovery document JSON.  The caller is responsible for calling
// server.Close() when done.
func makeDiscoveryServer(t *testing.T, doc any, status int) *httptest.Server {
	t.Helper()

	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshaling discovery doc: %v", err)
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration") {
			http.NotFound(w, r)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(body) //nolint:errcheck
	}))
}

// validDiscoveryDoc returns a minimal valid discovery document.
func validDiscoveryDoc(baseURL string) map[string]any {
	return map[string]any{
		"issuer":                 baseURL,
		"authorization_endpoint": baseURL + "/oauth2/authorize",
		"token_endpoint":         baseURL + "/oauth2/token",
		"userinfo_endpoint":      baseURL + "/oauth2/userinfo",
		"jwks_uri":               baseURL + "/.well-known/jwks.json",
		"grant_types_supported":  []string{"authorization_code", "client_credentials"},
		"response_types_supported": []string{"code"},
	}
}

// TestDiscoverEndpointsSuccess verifies that a valid discovery document is
// fetched, parsed, and cached correctly.
func TestDiscoverEndpointsSuccess(t *testing.T) {
	resetDiscoveryCache()

	var srvURL string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration") {
			http.NotFound(w, r)

			return
		}

		doc := validDiscoveryDoc(srvURL)
		body, _ := json.Marshal(doc)

		w.Header().Set("Content-Type", "application/json")
		w.Write(body) //nolint:errcheck
	}))
	defer srv.Close()

	srvURL = srv.URL

	doc, err := discoverEndpoints(srv.URL)
	if err != nil {
		t.Fatalf("discoverEndpoints() error: %v", err)
	}

	if doc.Issuer != srv.URL {
		t.Errorf("Issuer = %q, want %q", doc.Issuer, srv.URL)
	}

	if doc.JWKSUri == "" {
		t.Error("JWKSUri should not be empty")
	}

	if doc.TokenEndpoint == "" {
		t.Error("TokenEndpoint should not be empty")
	}

	if doc.AuthorizationEndpoint == "" {
		t.Error("AuthorizationEndpoint should not be empty")
	}

	resetDiscoveryCache()
}

// TestDiscoverEndpointsCache verifies that a second call returns the cached
// document without making another HTTP request.
func TestDiscoverEndpointsCache(t *testing.T) {
	resetDiscoveryCache()

	requestCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		doc := validDiscoveryDoc("http://" + r.Host)
		body, _ := json.Marshal(doc)

		w.Header().Set("Content-Type", "application/json")
		w.Write(body) //nolint:errcheck
	}))
	defer srv.Close()

	resetDiscoveryCache()

	// First call — should make one HTTP request.
	if _, err := discoverEndpoints(srv.URL); err != nil {
		t.Fatalf("first discoverEndpoints() error: %v", err)
	}

	if requestCount != 1 {
		t.Errorf("expected 1 HTTP request after first call, got %d", requestCount)
	}

	// Second call — should use the cache.
	if _, err := discoverEndpoints(srv.URL); err != nil {
		t.Fatalf("second discoverEndpoints() error: %v", err)
	}

	if requestCount != 1 {
		t.Errorf("expected still 1 HTTP request after cached call, got %d", requestCount)
	}
}

// TestDiscoverEndpointsMissingIssuer verifies that a document without the
// "issuer" field returns an error.
func TestDiscoverEndpointsMissingIssuer(t *testing.T) {
	resetDiscoveryCache()

	doc := map[string]any{
		// issuer intentionally omitted
		"token_endpoint": "http://example.com/token",
		"jwks_uri":       "http://example.com/jwks",
	}

	srv := makeDiscoveryServer(t, doc, http.StatusOK)
	defer srv.Close()

	resetDiscoveryCache()

	_, err := discoverEndpoints(srv.URL)
	if err == nil {
		t.Fatal("discoverEndpoints() should have returned an error for missing issuer")
	}

	if !strings.Contains(err.Error(), "issuer") {
		t.Errorf("error should mention 'issuer', got: %v", err)
	}
}

// TestDiscoverEndpointsMissingJWKSUri verifies that a document without the
// "jwks_uri" field returns an error.
func TestDiscoverEndpointsMissingJWKSUri(t *testing.T) {
	resetDiscoveryCache()

	doc := map[string]any{
		"issuer":         "http://example.com",
		"token_endpoint": "http://example.com/token",
		// jwks_uri intentionally omitted
	}

	srv := makeDiscoveryServer(t, doc, http.StatusOK)
	defer srv.Close()

	resetDiscoveryCache()

	_, err := discoverEndpoints(srv.URL)
	if err == nil {
		t.Fatal("discoverEndpoints() should have returned an error for missing jwks_uri")
	}

	if !strings.Contains(err.Error(), "jwks_uri") {
		t.Errorf("error should mention 'jwks_uri', got: %v", err)
	}
}

// TestDiscoverEndpointsMissingTokenEndpoint verifies that a document without
// the "token_endpoint" field returns an error.
func TestDiscoverEndpointsMissingTokenEndpoint(t *testing.T) {
	resetDiscoveryCache()

	doc := map[string]any{
		"issuer":   "http://example.com",
		"jwks_uri": "http://example.com/jwks",
		// token_endpoint intentionally omitted
	}

	srv := makeDiscoveryServer(t, doc, http.StatusOK)
	defer srv.Close()

	resetDiscoveryCache()

	_, err := discoverEndpoints(srv.URL)
	if err == nil {
		t.Fatal("discoverEndpoints() should have returned an error for missing token_endpoint")
	}

	if !strings.Contains(err.Error(), "token_endpoint") {
		t.Errorf("error should mention 'token_endpoint', got: %v", err)
	}
}

// TestDiscoverEndpointsHTTPError verifies that an HTTP error response from the
// discovery endpoint returns an error.
func TestDiscoverEndpointsHTTPError(t *testing.T) {
	resetDiscoveryCache()

	srv := makeDiscoveryServer(t, map[string]any{}, http.StatusInternalServerError)
	defer srv.Close()

	resetDiscoveryCache()

	_, err := discoverEndpoints(srv.URL)
	if err == nil {
		t.Fatal("discoverEndpoints() should have returned an error for HTTP 500")
	}
}

// TestDiscoverEndpointsTrailingSlash verifies that a provider URL with a
// trailing slash is handled correctly.
func TestDiscoverEndpointsTrailingSlash(t *testing.T) {
	resetDiscoveryCache()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)

			return
		}

		doc := validDiscoveryDoc("http://" + r.Host)
		body, _ := json.Marshal(doc)

		w.Header().Set("Content-Type", "application/json")
		w.Write(body) //nolint:errcheck
	}))
	defer srv.Close()

	resetDiscoveryCache()

	// Pass a URL with a trailing slash — should still construct the path correctly.
	_, err := discoverEndpoints(srv.URL + "/")
	if err != nil {
		t.Fatalf("discoverEndpoints() with trailing slash error: %v", err)
	}
}
