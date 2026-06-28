package oauth

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/tucats/ego/internal/caches"
	"github.com/tucats/ego/internal/language/tokens"
)

// ---- OAUTH-H2: blacklist check on JWT cache hits ----

// TestValidateJWT_CacheHit_BlacklistedJTI verifies that ValidateJWT rejects a
// cached JWT entry whose JTI has been added to the revocation blacklist since
// the token was last validated (OAUTH-H2).
//
// Before this fix, the cache hit path returned the cached user/permissions
// without consulting the blacklist, so a revoked token remained accepted for
// the full cache TTL — up to one hour by default.
//
// The test directly pre-populates caches.OAuthJWTCache with a *JWTCacheEntry
// to exercise the cache hit branch in isolation, without needing a live JWKS
// or IdP endpoint.
func TestValidateJWT_CacheHit_BlacklistedJTI(t *testing.T) {
	// Configure a blacklist backed by a temporary SQLite database.
	// The connection string must carry the "sqlite3://" scheme prefix because
	// resources.Open uses egostrings.FindScheme to detect the driver.
	dir := t.TempDir()
	dbPath := "sqlite3://" + filepath.Join(dir, "blacklist.db")

	if err := tokens.SetDatabasePath(dbPath); err != nil {
		t.Skipf("blacklist database setup failed (%v) — skipping", err)
	}

	t.Cleanup(func() {
		tokens.Close()
		_ = tokens.SetDatabasePath("")
	})

	const (
		fakeToken = "header.payload.signature" // three-segment string that IsJWT accepts
		jti       = "test-jti-blacklisted-12345"
		user      = "alice"
	)

	// Pre-populate the JWT result cache so ValidateJWT hits the fast path.
	caches.Add(caches.OAuthJWTCache, fakeToken, &JWTCacheEntry{
		User:        user,
		Permissions: []string{"ego.logon"},
		Expires:     time.Now().Add(time.Hour),
		JTI:         jti,
	})

	t.Cleanup(func() {
		caches.Delete(caches.OAuthJWTCache, fakeToken)
	})

	// Blacklist the JTI, simulating a POST /oauth2/revoke call.
	if err := tokens.Blacklist(jti); err != nil {
		t.Fatalf("Blacklist: %v", err)
	}

	// ValidateJWT must detect the blacklisted JTI on the cache hit and return
	// an error rather than the cached user/permissions.
	gotUser, gotPerms, err := ValidateJWT(1, fakeToken)

	if err == nil {
		t.Error("expected an error for a blacklisted JTI on cache hit")
	}

	if gotUser != "" {
		t.Errorf("expected empty user for revoked token, got %q", gotUser)
	}

	if gotPerms != nil {
		t.Errorf("expected nil permissions for revoked token, got %v", gotPerms)
	}

	// The cache entry must be evicted so the blacklist check is not bypassed
	// on subsequent calls to ValidateJWT with the same token string.
	if _, found := caches.Find(caches.OAuthJWTCache, fakeToken); found {
		t.Error("expected the JWT cache entry to be evicted after blacklist detection")
	}
}

// TestValidateJWT_CacheHit_NotBlacklisted verifies that ValidateJWT accepts a
// cached JWT entry whose JTI has NOT been revoked, returning the cached
// user and permissions without re-parsing the JWT (OAUTH-H2 positive path).
//
// This test ensures the fix does not regress the normal cache-hit behavior.
func TestValidateJWT_CacheHit_NotBlacklisted(t *testing.T) {
	// Configure an empty blacklist database (nothing will be blacklisted).
	dir := t.TempDir()
	dbPath := "sqlite3://" + filepath.Join(dir, "blacklist_empty.db")

	if err := tokens.SetDatabasePath(dbPath); err != nil {
		t.Skipf("blacklist database setup failed (%v) — skipping", err)
	}

	t.Cleanup(func() {
		tokens.Close()
		_ = tokens.SetDatabasePath("")
	})

	const (
		fakeToken2 = "hdr.pay.sig"
		jti2       = "test-jti-not-blacklisted-67890"
		user2      = "bob"
	)

	caches.Add(caches.OAuthJWTCache, fakeToken2, &JWTCacheEntry{
		User:        user2,
		Permissions: []string{"ego.logon", "ego.tables.read"},
		Expires:     time.Now().Add(time.Hour),
		JTI:         jti2,
	})

	t.Cleanup(func() {
		caches.Delete(caches.OAuthJWTCache, fakeToken2)
	})

	gotUser, gotPerms, err := ValidateJWT(1, fakeToken2)

	if err != nil {
		t.Errorf("unexpected error for non-blacklisted cached JWT: %v", err)
	}

	if gotUser != user2 {
		t.Errorf("user = %q, want %q", gotUser, user2)
	}

	if len(gotPerms) != 2 {
		t.Errorf("permissions = %v, want 2 entries", gotPerms)
	}

	// The cache entry must still be present after a successful hit.
	if _, found := caches.Find(caches.OAuthJWTCache, fakeToken2); !found {
		t.Error("valid cache entry should remain after a successful hit")
	}
}

// TestValidateJWT_CacheHit_NoJTI verifies that a cached JWT entry without a
// JTI field skips the blacklist check and is accepted normally (OAUTH-H2
// edge case).
//
// Some IdPs omit the "jti" claim.  In that case we cannot perform a
// meaningful blacklist lookup, so we accept the cached result as-is.
func TestValidateJWT_CacheHit_NoJTI(t *testing.T) {
	const (
		fakeToken3 = "a.b.c"
		user3      = "carol"
	)

	caches.Add(caches.OAuthJWTCache, fakeToken3, &JWTCacheEntry{
		User:        user3,
		Permissions: []string{"ego.logon"},
		Expires:     time.Now().Add(time.Hour),
		JTI:         "", // no jti claim
	})

	t.Cleanup(func() {
		caches.Delete(caches.OAuthJWTCache, fakeToken3)
	})

	gotUser, _, err := ValidateJWT(1, fakeToken3)

	if err != nil {
		t.Errorf("unexpected error for cached JWT with no JTI: %v", err)
	}

	if gotUser != user3 {
		t.Errorf("user = %q, want %q", gotUser, user3)
	}
}

// TestValidateJWT_CacheHit_ExpiredEntry verifies that an expired cache entry
// is evicted rather than returned, forcing a full re-validation on the next
// call.  This is pre-existing behavior preserved by the OAUTH-H2 change.
func TestValidateJWT_CacheHit_ExpiredEntry(t *testing.T) {
	const fakeToken4 = "x.y.z"

	caches.Add(caches.OAuthJWTCache, fakeToken4, &JWTCacheEntry{
		User:        "dave",
		Permissions: []string{"ego.logon"},
		Expires:     time.Now().Add(-time.Minute), // already expired
		JTI:         "some-jti",
	})

	t.Cleanup(func() {
		caches.Delete(caches.OAuthJWTCache, fakeToken4)
	})

	// The expired entry must not be returned.  ValidateJWT will try to validate
	// against the JWKS and fail (no JWKS configured in tests); the important
	// thing is that it does NOT return the cached user.
	gotUser, _, _ := ValidateJWT(1, fakeToken4)

	if gotUser == "dave" {
		t.Error("expired cache entry should not be returned by ValidateJWT")
	}

	// The expired entry should have been evicted during the cache-hit check.
	if _, found := caches.Find(caches.OAuthJWTCache, fakeToken4); found {
		t.Error("expected expired cache entry to be evicted")
	}
}
