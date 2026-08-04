# OAUTH-H2 — Revoked JWT tokens bypass the RS validation cache

**Affected file:** `server/oauth/oauth.go:204` — `ValidateJWT()`

```go
if v, found := caches.Find(caches.OAuthJWTCache, tokenStr); found {
    entry, ok := v.(*JWTCacheEntry)
    if ok && time.Now().Before(entry.Expires) {
        return entry.User, entry.Permissions, nil  // ← no blacklist check
    }
```

**Description:**
When a JWT is found in `caches.OAuthJWTCache`, it is returned as valid after a
single `time.Now().Before(entry.Expires)` check. The JTI blacklist populated by
`POST /oauth2/revoke` is never consulted on a cache hit. A token that has been
explicitly revoked can therefore continue to authenticate all RS requests for up
to the configured JWKS cache TTL (default 1 hour) after revocation.

By contrast, the AS's own `UserinfoHandler` does perform a blacklist check on
every request (`tokens.IsIDBlacklisted(claims.ID)`), so the inconsistency is
not caused by a missing API — the call is simply absent from the hot cache-hit
path in `ValidateJWT`.

This is analogous to the cached-token expiry bypass described in LOGIN-M3.

**Recommendation:**
Store the JTI (`claims.ID`) inside `JWTCacheEntry` and check
`tokens.IsIDBlacklisted(entry.JTI)` before returning from the cache-hit branch.
A blacklisted JTI should result in immediate cache eviction and a validation
error, identical to the behavior of a post-expiry entry.

**Resolution (May 2026):**
`JWTCacheEntry` gained a `JTI string` field that stores the JWT ID claim.
`ValidateJWT` now calls `tokens.IsIDBlacklisted(entry.JTI)` on every cache hit
before returning. A positive blacklist result evicts the cache entry immediately
and returns an error; the caller is denied. `ValidateJWT` also stores
`JTI: claims.ID` when writing new cache entries. New log message key
`oauth.rs.jwt.revoked` added to all three language files. Tests in
`server/oauth/oauth_cache_test.go`.

