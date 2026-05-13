package router

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/caches"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/tokens"
)

// authTestFile is the temporary user-database file used by the auth service
// during cache authentication tests.
var authTestFile = filepath.Join(os.TempDir(), fmt.Sprintf("ego_auth_cache_test-%s.json", uuid.New().String()))

// TestMain initializes a minimal file-backed auth service so that the
// auth.GetPermissions calls inside Authenticate do not panic when the
// test users (cached only, not in the database) are not found.
// The service is torn down after all tests in this package complete.
func TestMain(m *testing.M) {
	svc, err := auth.NewFileService(authTestFile, defs.DefaultAdminUsername, defs.DefaultAdminPassword)
	if err != nil {
		fmt.Fprintf(os.Stderr, "auth_test: failed to create test auth service: %v\n", err)
		os.Exit(1)
	}

	auth.AuthService = svc

	code := m.Run()

	_ = os.Remove(authTestFile)
	os.Exit(code)
}

// bearerRequest constructs an HTTP request carrying the given token string as a
// Bearer credential. The token does not need to be a valid encrypted token for
// cache-hit tests; any non-empty string works as a cache key.
func bearerRequest(t *testing.T, token string) *http.Request {
	t.Helper()

	r, err := http.NewRequest(http.MethodGet, "/test", nil)
	if err != nil {
		t.Fatalf("bearerRequest: %v", err)
	}

	r.Header.Set("Authorization", "Bearer "+token)

	return r
}

// purgeTokenCache removes all entries from the token cache between tests.
func purgeTokenCache() {
	caches.Purge(caches.TokenCache)
}

// TestAuthenticate_CacheHit_ValidToken verifies that a non-expired token found
// in the cache is accepted without re-decryption, and that the session is
// populated with the correct username.
func TestAuthenticate_CacheHit_ValidToken(t *testing.T) {
	purgeTokenCache()

	const (
		tokenKey = "valid-token-abc123"
		wantUser = "alice"
	)

	tok := &tokens.Token{
		Name:    wantUser,
		TokenID: uuid.New(),
		Expires: time.Now().Add(time.Hour), // well in the future
	}

	caches.Add(caches.TokenCache, tokenKey, tok)

	s := &Session{ID: 1}
	result := s.Authenticate(bearerRequest(t, tokenKey))

	if !result.Authenticated {
		t.Error("expected Authenticated == true for a valid cached token")
	}

	if result.User != wantUser {
		t.Errorf("User = %q, want %q", result.User, wantUser)
	}

	// Token should still be in the cache after a successful hit.
	if _, found := caches.Find(caches.TokenCache, tokenKey); !found {
		t.Error("expected token to remain in cache after a valid hit")
	}
}

// TestAuthenticate_CacheHit_ExpiredToken verifies that a token whose Expires
// timestamp has already passed is rejected and evicted from the cache, even
// though it is present in the cache.
func TestAuthenticate_CacheHit_ExpiredToken(t *testing.T) {
	purgeTokenCache()

	const tokenKey = "expired-token-xyz789"

	tok := &tokens.Token{
		Name:    "bob",
		TokenID: uuid.New(),
		Expires: time.Now().Add(-time.Minute), // already expired
	}

	caches.Add(caches.TokenCache, tokenKey, tok)

	s := &Session{ID: 2}
	result := s.Authenticate(bearerRequest(t, tokenKey))

	if result.Authenticated {
		t.Error("expected Authenticated == false for an expired cached token")
	}

	// The expired entry must have been evicted.
	if _, found := caches.Find(caches.TokenCache, tokenKey); found {
		t.Error("expected expired token to be evicted from cache")
	}
}

// TestAuthenticate_CacheHit_RemoteAuthorityToken verifies that a synthetic
// Token entry produced for a remote-authority server (Expires is the zero
// value) is not rejected by the expiry check. A zero Expires means "no local
// expiry information available; defer to the authority".
func TestAuthenticate_CacheHit_RemoteAuthorityToken(t *testing.T) {
	purgeTokenCache()

	const (
		tokenKey = "remote-authority-token-def456"
		wantUser = "carol"
	)

	// Synthetic entry: only Name is set; Expires is zero (remote authority case).
	tok := &tokens.Token{
		Name: wantUser,
		// Expires intentionally zero
	}

	caches.Add(caches.TokenCache, tokenKey, tok)

	s := &Session{ID: 3}
	result := s.Authenticate(bearerRequest(t, tokenKey))

	if !result.Authenticated {
		t.Error("expected Authenticated == true for a remote-authority cached token with zero Expires")
	}

	if result.User != wantUser {
		t.Errorf("User = %q, want %q", result.User, wantUser)
	}

	// Token should remain in the cache.
	if _, found := caches.Find(caches.TokenCache, tokenKey); !found {
		t.Error("expected remote-authority token to remain in cache after a valid hit")
	}
}

// TestAuthenticate_CacheHit_ExpiredThenMiss verifies that after an expired
// token is evicted, the same token key produces an unauthenticated session
// on a second request (no stale acceptance after eviction).
func TestAuthenticate_CacheHit_ExpiredThenMiss(t *testing.T) {
	purgeTokenCache()

	const tokenKey = "expired-then-miss-token"

	tok := &tokens.Token{
		Name:    "dave",
		TokenID: uuid.New(),
		Expires: time.Now().Add(-time.Second),
	}

	caches.Add(caches.TokenCache, tokenKey, tok)

	// First request: expired token is evicted.
	s1 := &Session{ID: 4}
	r1 := s1.Authenticate(bearerRequest(t, tokenKey))

	if r1.Authenticated {
		t.Error("first request: expected unauthenticated for expired token")
	}

	// Second request: cache is empty; TokenUnwrap will fail on the invalid key
	// string, so the session must again be unauthenticated.
	s2 := &Session{ID: 5}
	r2 := s2.Authenticate(bearerRequest(t, tokenKey))

	if r2.Authenticated {
		t.Error("second request: expected unauthenticated after cache eviction")
	}
}
