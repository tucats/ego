# OAUTH-M7 — Every JWT with an unknown `kid` triggers an unconditional JWKS refresh

**Affected file:** `server/oauth/jwks.go:200-209` — `keyByID()`

```go
// Try the cache first if it is fresh.
if len(keys) > 0 && age < ttl {
    if key := findKeyByID(keys, kid); key != nil {
        return key, nil
    }
}

// Cache miss or stale — refresh and try again.
if err := refreshJWKS(jwksURL); err != nil { ...
```

**Description:**
`keyByID` refreshes the JWKS unconditionally whenever the requested `kid` is not
found in the local cache, including when the cache is still fresh (`age < ttl`).
The intent is to handle key rotation gracefully: if the IdP rotates its signing
key, the first request bearing the new kid triggers a fetch. However, the guard
condition only prevents a refresh when the same kid is already in the cache; it
does not prevent repeated refreshes when a novel unknown kid is presented on every
request.

An attacker who can present Bearer tokens (even syntactically valid ones that
will ultimately fail signature verification) with a unique, non-existent `kid`
header on each request forces one JWKS network fetch per request. With many
concurrent such requests:

- Each `refreshJWKS` call makes a round-trip to the IdP that can last up to
  10 seconds (the `idpClient` timeout), blocking the handling goroutine.
- Many concurrent fetches starve the server's goroutine pool.
- The IdP is hammered with repeated JWKS requests, potentially triggering IdP-
  side rate limiting that blocks legitimate key lookups from the same server IP.

There is no throttle, cooldown, or minimum inter-refresh interval guarding the
cache-fresh-but-kid-unknown branch.

**Recommendation:**
Track the timestamp of the most recent JWKS refresh triggered by a cache miss and
refuse to refresh more than once per configurable minimum interval (e.g., 30
seconds) in the cache-is-fresh-but-kid-missing branch. A package-level
`lastMissRefresh time.Time` protected by a mutex is sufficient:

```go
var missRefresh struct {
    mu   sync.Mutex
    last time.Time
}
const minMissRefreshInterval = 30 * time.Second

// Inside keyByID, after the cache-fresh-kid-missing case:
missRefresh.mu.Lock()
if time.Since(missRefresh.last) < minMissRefreshInterval {
    missRefresh.mu.Unlock()
    return nil, errors.New(errors.ErrJWKSKeyNotFound).Context(kid)
}
missRefresh.last = time.Now()
missRefresh.mu.Unlock()
// then call refreshJWKS
```

The key-rotation use case is fully preserved: a genuine rotation will succeed
on the first unknown-kid request; subsequent requests within the 30-second window
will see the refreshed cache and find the new key (or correctly fail if the kid
is genuinely absent).

**Resolution (June 2026):**
`server/oauth/jwks.go` received three coordinated changes:

1. **`missRefresh` state variable** — a package-level struct with a `sync.Mutex`
   and a `last time.Time` field tracks when the most recent miss-triggered JWKS
   refresh occurred.

2. **`minMissRefreshInterval` constant** — set to 30 seconds.  A miss-triggered
   refresh is allowed at most once per this window.

3. **`keyByID` cooldown logic** — a `freshCacheMiss` boolean is set to `true`
   when the cache is within its TTL but the requested kid is absent.  When
   `freshCacheMiss` is true, the function acquires `missRefresh.mu` and checks
   whether `time.Since(missRefresh.last) < minMissRefreshInterval`.  If the
   cooldown is still active, it returns `ErrJWKSKeyNotFound` immediately without
   a network call.  If the cooldown has expired, it records `missRefresh.last =
   time.Now()` and proceeds to `refreshJWKS`.  Stale or empty caches are always
   refreshed without consulting the cooldown (the `freshCacheMiss` guard ensures
   the cooldown only applies to the fresh-cache miss path).

A `resetMissRefresh()` test helper was added to `jwks.go` alongside the existing
`resetJWKSCache()`.  Five tests were added to `server/oauth/medium_test.go`:
`TestKeyByID_FirstMissTriggersFetch` (first unknown-kid miss fires one refresh),
`TestKeyByID_CooldownBlocksSecondMiss` (second miss within cooldown fires zero
refreshes and returns `ErrJWKSKeyNotFound`),
`TestKeyByID_CooldownExpiryAllowsRefresh` (after cooldown window, refresh fires
again — time-warp via direct mutation of `missRefresh.last`),
`TestKeyByID_StaleCacheBypassesCooldown` (stale cache always refreshes regardless
of the cooldown), and `TestKeyByID_KnownKidInFreshCacheHitsNoNetwork` (fast path
regression — a known kid in a fresh cache still returns immediately with no fetch).

