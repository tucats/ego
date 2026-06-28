// Package authserver — revoke_test.go covers the OAUTH-M3 security fix:
// the POST /oauth2/revoke endpoint now accepts client credentials in the HTTP
// Basic Authorization header in addition to form-encoded body fields.
package authserver

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/ego/internal/router"
)

// ─────────────────────────────────────────────────────────────────────────────
// OAUTH-M3: Basic Auth at the revocation endpoint
// ─────────────────────────────────────────────────────────────────────────────

// setupRevokeClients registers a confidential client (with a secret) for
// revocation tests.  It loads the client from a JSON file so the plaintext
// secret is properly bcrypt-hashed by loadClients, exactly as it would be in
// production.
//
// The returned cleanup function removes the client from the registry.
func setupRevokeClients(t *testing.T, clientID, clientSecret string) {
	t.Helper()

	dir := t.TempDir()
	clientFile := filepath.Join(dir, "revoke_clients.json")

	record := []map[string]any{
		{
			"client_id":     clientID,
			"client_secret": clientSecret,
			"redirect_uris": []string{"https://example.com/cb"},
			"grant_types":   []string{"authorization_code"},
			"scopes":        []string{"openid"},
		},
	}

	data, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshaling client file: %v", err)
	}

	if err := os.WriteFile(clientFile, data, 0600); err != nil {
		t.Fatalf("writing client file: %v", err)
	}

	if err := loadClients(clientFile); err != nil {
		t.Fatalf("loadClients: %v", err)
	}

	t.Cleanup(func() { clients = nil })
}

// basicAuthHeader builds a valid HTTP Basic Authorization header value for the
// given credentials.
//
// HTTP Basic Auth encodes the credentials as:
//
//	"Basic " + base64("username:password")
//
// This is the standard mechanism defined in RFC 7617.  Most HTTP client
// libraries generate this header automatically; here we build it manually so
// the test is self-contained and explicit.
func basicAuthHeader(clientID, secret string) string {
	// base64-encode "clientID:secret" with standard encoding (not URL-safe).
	creds := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + secret))

	return "Basic " + creds
}

// postRevoke drives a POST to RevokeHandler with the given request body and
// optional Authorization header, and returns the HTTP status code.
func postRevoke(t *testing.T, body url.Values, authHeader string) int {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/oauth2/revoke",
		strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	w := httptest.NewRecorder()
	sess := &router.Session{ID: 1}

	return RevokeHandler(sess, w, req)
}

// TestRevokeHandler_BasicAuth_Accepted verifies that a confidential client can
// authenticate at POST /oauth2/revoke using the HTTP Basic Authorization header
// (OAUTH-M3).
//
// Before this fix, the handler read credentials only from the form body.
// RFC 7009 §2.1 requires the revocation endpoint to support the same client-
// authentication mechanism as the token endpoint, which accepts both Basic Auth
// and form-encoded credentials.
//
// We send a missing/empty token so the handler hits the "token absent → 200 OK"
// path after authenticating the client.  A 401 response means the client was
// not authenticated, proving the fix is needed.
func TestRevokeHandler_BasicAuth_Accepted(t *testing.T) {
	const (
		clientID = "revoke-basic-client"
		secret   = "supersecret"
	)

	setupRevokeClients(t, clientID, secret)

	// Put the credentials in the Authorization header (Basic Auth), NOT in
	// the form body.  This is the OAUTH-M3 path.
	form := url.Values{}
	// No client_id or client_secret in the form — they come from the header.
	// token is empty so the handler returns 200 OK after authenticating.

	status := postRevoke(t, form, basicAuthHeader(clientID, secret))

	// 401 = client was rejected (bug still present).
	// 400 = client authenticated but request was malformed.
	// 200 = client authenticated, no token to revoke — the expected result.
	if status == http.StatusUnauthorized {
		t.Errorf("RevokeHandler returned 401 for valid Basic Auth credentials — OAUTH-M3 fix is not working")
	}

	// Any non-401 result means the client was authenticated.  The exact code
	// depends on the token field; we accept 200 and 400.
	if status != http.StatusOK && status != http.StatusBadRequest {
		t.Errorf("unexpected status %d; want 200 or 400 (client authenticated)", status)
	}
}

// TestRevokeHandler_FormCredentials_StillAccepted verifies that existing
// clients that supply credentials in the form body continue to work after the
// OAUTH-M3 fix (backward compatibility).
//
// validateBasicAuth (which RevokeHandler now calls) tries the Authorization
// header first and falls back to form fields when the header is absent.  This
// test exercises the fallback path.
func TestRevokeHandler_FormCredentials_StillAccepted(t *testing.T) {
	const (
		clientID = "revoke-form-client"
		secret   = "formsecret"
	)

	setupRevokeClients(t, clientID, secret)

	// Put the credentials in the form body (the pre-fix behavior).
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", secret)

	// No Authorization header.
	status := postRevoke(t, form, "")

	if status == http.StatusUnauthorized {
		t.Error("RevokeHandler should still accept form-encoded credentials (OAUTH-M3 regression)")
	}
}

// TestRevokeHandler_WrongSecret_Rejected verifies that a client presenting the
// wrong secret in the Basic Auth header is rejected with 401.
//
// This test guards against a regression where the fix might accidentally
// accept any Basic Auth header without validating the secret.
func TestRevokeHandler_WrongSecret_Rejected(t *testing.T) {
	const (
		clientID    = "revoke-reject-client"
		goodSecret  = "correctsecret"
		wrongSecret = "wrongsecret"
	)

	setupRevokeClients(t, clientID, goodSecret)

	form := url.Values{}
	status := postRevoke(t, form, basicAuthHeader(clientID, wrongSecret))

	if status != http.StatusUnauthorized {
		t.Errorf("RevokeHandler should return 401 for wrong Basic Auth secret, got %d", status)
	}
}

// TestRevokeHandler_UnknownClient_Rejected verifies that a client_id that does
// not exist in the registry is rejected regardless of whether the credentials
// are supplied via Basic Auth or form fields.
func TestRevokeHandler_UnknownClient_Rejected(t *testing.T) {
	// Start with an empty client registry.
	clients = nil

	t.Cleanup(func() { clients = nil })

	form := url.Values{}
	status := postRevoke(t, form, basicAuthHeader("nonexistent", "secret"))

	if status != http.StatusUnauthorized && status != http.StatusBadRequest {
		t.Errorf("unknown client should be rejected (want 401 or 400), got %d", status)
	}
}

// TestRevokeHandler_MissingClientID_Rejected verifies that a request with no
// client_id at all (neither in the header nor in the form) returns an error
// rather than panicking or accepting.
func TestRevokeHandler_MissingClientID_Rejected(t *testing.T) {
	clients = nil
	
	t.Cleanup(func() { clients = nil })

	form := url.Values{}
	// No Authorization header and no client_id form field.
	status := postRevoke(t, form, "")

	// Should be 400 (missing client_id) or 401 (invalid credentials).
	if status == http.StatusOK {
		t.Error("RevokeHandler should not return 200 when client_id is absent")
	}
}

// TestRevokeHandler_PublicClient_BasicAuth_Accepted verifies that a public
// client (no secret) can authenticate at the revocation endpoint using Basic
// Auth with an empty password.
//
// Public clients have no ClientSecretHash.  validateClientSecret returns true
// for any secret (including an empty string) when the hash is empty.
func TestRevokeHandler_PublicClient_BasicAuth_Accepted(t *testing.T) {
	const clientID = "revoke-public-client"

	// Register a public client directly — no client_secret field.
	clients = []OAuthClient{{
		ClientID:     clientID,
		RedirectURIs: []string{"https://public.example.com/cb"},
		GrantTypes:   []string{"authorization_code"},
		Scopes:       []string{"openid"},
		// ClientSecretHash intentionally empty — public client.
	}}

	t.Cleanup(func() { clients = nil })

	// Basic Auth with an empty password — allowed for public clients.
	form := url.Values{}
	status := postRevoke(t, form, basicAuthHeader(clientID, ""))

	if status == http.StatusUnauthorized {
		t.Errorf("public client with empty Basic Auth secret should be accepted, got 401")
	}
}

// Ensure the fmt import is used so the build does not fail if no test uses it.
var _ = fmt.Sprintf
