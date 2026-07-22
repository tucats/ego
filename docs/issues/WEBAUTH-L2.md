# WEBAUTH-L2 — Cache item expiration only refreshed when CACHE logging is active

**Affected file:** `caches/find.go:46`

**Description:**  
The sliding-expiration update inside `caches.Find` is nested inside
`if ui.IsActive(ui.CacheLogger)`. When CACHE logging is disabled (the normal
production state), cache items are never given a refreshed TTL on access — they
always expire at their original creation time. When CACHE logging is enabled,
any access extends an item's lifetime. This means server behavior changes
silently based on a logging flag. For WebAuthn challenge nonces the effect is
harmless (nonces are deleted immediately after the first successful
`loadChallenge` call), but the pattern is fragile for other cache classes such
as `TokenCache` and `AuthCache`.

**Resolution:**  
The expiration-refresh update in `caches/find.go` has been moved outside the
`if ui.IsActive(ui.CacheLogger)` block. It now executes on every cache hit
regardless of log level.

