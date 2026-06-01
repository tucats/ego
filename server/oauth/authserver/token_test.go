package authserver

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tucats/ego/router"
)

// ---- helpers ----

// setupTokenTestKey generates a fresh EC signing key in a temp directory and
// sets asGlobalConfig to a minimal configuration so that token creation works.
// The cleanup function restores asGlobalConfig to its zero value.
func setupTokenTestKey(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	if err := loadOrGenerateKey(filepath.Join(dir, "test.pem")); err != nil {
		t.Fatalf("key setup: %v", err)
	}

	asGlobalConfig = asConfig{
		Issuer:          "https://ego.test",
		TokenExpiration: time.Hour,
	}

	t.Cleanup(func() { asGlobalConfig = asConfig{} })
}

// setupPublicClient registers a minimal public OAuth2 client — one without a
// client_secret_hash — and cleans up after the test.
func setupPublicClient(t *testing.T, clientID, redirectURI string) {
	t.Helper()

	clients = []OAuthClient{{
		ClientID:     clientID,
		RedirectURIs: []string{redirectURI},
		GrantTypes:   []string{"authorization_code", "refresh_token"},
		Scopes:       []string{"openid"},
		// ClientSecretHash intentionally empty — marks this as a public client.
	}}

	t.Cleanup(func() { clients = nil })
}

// postToToken is a helper that drives a POST form request through TokenHandler
// and returns the HTTP status code.
func postToToken(t *testing.T, form url.Values) int {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/oauth2/token",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	sess := &router.Session{ID: 99}

	return TokenHandler(sess, w, req)
}

// ---- OAUTH-H3: PKCE required for public clients ----

// TestTokenHandler_PublicClient_RequiresPKCE verifies that a public client
// (no ClientSecretHash) cannot exchange an authorization code that was issued
// without a code_challenge (OAUTH-H3).
//
// Per RFC 9700 §2.1.1, all public clients MUST use PKCE.  A code without a
// bound code_challenge can be replayed by any attacker who learns the
// authorization code, because there is no proof-of-possession requirement —
// the client_id is public by definition.
func TestTokenHandler_PublicClient_RequiresPKCE(t *testing.T) {
	setupTokenTestKey(t)
	setupPublicClient(t, "publicapp", "https://public.example.com/cb")

	// Simulate an authorization request where the public client omitted PKCE
	// by storing a PendingAuthorization with an empty CodeChallenge.
	code, err := generateCode()
	if err != nil {
		t.Fatalf("generateCode: %v", err)
	}

	storeCode(code, PendingAuthorization{
		ClientID:    "publicapp",
		RedirectURI: "https://public.example.com/cb",
		Scopes:      []string{"openid"},
		Username:    "alice",
		IssuedAt:    time.Now(),
		// CodeChallenge intentionally empty — public client skipped PKCE.
	})

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", "publicapp")
	form.Set("code", code)
	form.Set("redirect_uri", "https://public.example.com/cb")
	// code_verifier intentionally absent.

	if status := postToToken(t, form); status != http.StatusBadRequest {
		t.Errorf("expected 400 for public client without PKCE, got %d", status)
	}
}

// TestTokenHandler_PublicClient_WithPKCE_Succeeds verifies that a public
// client that correctly used PKCE during authorization can exchange the code
// for tokens (OAUTH-H3 positive path).  The enforcement must reject only the
// flows that truly omitted PKCE, not all public-client exchanges.
func TestTokenHandler_PublicClient_WithPKCE_Succeeds(t *testing.T) {
	setupTokenTestKey(t)
	setupPublicClient(t, "pkceapp", "https://pkce.example.com/cb")

	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	// Compute S256 challenge inline: BASE64URL(SHA256(verifier)) — the same
	// formula used by verifyPKCE in codes.go.
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])

	code, err := generateCode()
	if err != nil {
		t.Fatalf("generateCode: %v", err)
	}

	storeCode(code, PendingAuthorization{
		ClientID:            "pkceapp",
		RedirectURI:         "https://pkce.example.com/cb",
		Scopes:              []string{"openid"},
		Username:            "bob",
		IssuedAt:            time.Now(),
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
	})

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", "pkceapp")
	form.Set("code", code)
	form.Set("redirect_uri", "https://pkce.example.com/cb")
	form.Set("code_verifier", verifier)

	if status := postToToken(t, form); status != http.StatusOK {
		t.Errorf("expected 200 for public client with correct PKCE, got %d", status)
	}
}

// TestTokenHandler_ConfidentialClient_NoPKCE_Allowed verifies that a
// confidential client (one that has a ClientSecretHash) is NOT required to
// use PKCE (OAUTH-H3 scope boundary).  The RFC 9700 mandate is specific to
// public clients; confidential clients may still omit PKCE.
func TestTokenHandler_ConfidentialClient_NoPKCE_Allowed(t *testing.T) {
	setupTokenTestKey(t)

	// Build the client file on disk so the plaintext secret is properly hashed
	// by loadClients (direct struct construction would leave ClientSecretHash empty).
	dir := t.TempDir()
	clientFile := filepath.Join(dir, "conf_clients.json")

	clientJSON := `[{
		"client_id":     "confclient",
		"client_secret": "secretpassword",
		"redirect_uris": ["https://conf.example.com/cb"],
		"grant_types":   ["authorization_code"],
		"scopes":        ["openid"]
	}]`

	if err := os.WriteFile(clientFile, []byte(clientJSON), 0600); err != nil {
		t.Fatalf("writing client file: %v", err)
	}

	if err := loadClients(clientFile); err != nil {
		t.Fatalf("loadClients: %v", err)
	}

	t.Cleanup(func() { clients = nil })

	code, _ := generateCode()
	storeCode(code, PendingAuthorization{
		ClientID:    "confclient",
		RedirectURI: "https://conf.example.com/cb",
		Scopes:      []string{"openid"},
		Username:    "carol",
		IssuedAt:    time.Now(),
		// CodeChallenge empty — confidential clients may omit PKCE.
	})

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", "confclient")
	form.Set("client_secret", "secretpassword")
	form.Set("code", code)
	form.Set("redirect_uri", "https://conf.example.com/cb")
	// No code_verifier — PKCE is optional for confidential clients.

	status := postToToken(t, form)

	// A 400 specifically from the PKCE requirement check would be a regression.
	// Other status codes (e.g. 500 from an unset auth service) are acceptable
	// for the purposes of this test, which only verifies the PKCE gate.
	if status == http.StatusBadRequest {
		t.Error("confidential client without PKCE should not be rejected by the PKCE gate")
	}
}
