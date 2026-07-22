# OAUTH-M5 — RS PKCE state store has no maximum size

**Affected file:** `server/oauth/state.go:75` — `newState()`

```go
stateStore.items[state] = &pendingState{ ... }
```

**Description:**
The `stateStore.items` map that holds pending PKCE states has no upper bound on
the number of entries. Every call to `GET /services/admin/oauth/authorize`
(or, transitively, to `AuthorizeRedirectHandler`) inserts a new entry. The
background goroutine in `Initialize()` purges entries older than 10 minutes every
10 minutes, but a flood of requests between purge cycles can fill the map beyond
what a single purge pass can evict. In the worst case an attacker can call the
authorize endpoint in a tight loop to exhaust memory, since each entry is created
without any per-IP or global cap.

This is analogous to the unbounded WebAuthn challenge cache issue described in
WEBAUTH-M1.

**Recommendation:**
Add a global cap on the number of pending state entries and enforce a per-IP rate
limit on calls to `AuthorizeRedirectHandler`. A cap of 500 concurrent pending
states is generous for normal usage:

```go
const maxPendingStates = 500

stateStore.mu.Lock()
if len(stateStore.items) >= maxPendingStates {
    stateStore.mu.Unlock()
    return "", "", fmt.Errorf("too many pending OAuth2 flows")
}
stateStore.items[state] = &pendingState{ ... }
stateStore.mu.Unlock()
```

Additionally, move the `purgeExpiredStates()` ticker to run more frequently
(e.g., every 2 minutes instead of 10) so the cap is only reached under genuine
sustained load.

**Resolution (May 2026):**
Three constants were added to `server/oauth/state.go`:
`statePurgeInterval = 2 * time.Minute` (the new ticker interval),
`maxPendingStates = 500` (the global cap), and the existing `stateMaxAge` is
unchanged (10 minutes).  `newState()` now acquires `stateStore.mu`, checks
`len(stateStore.items) >= maxPendingStates`, and returns an error before
inserting if the cap is reached.  The check and insert share a single lock
acquisition, eliminating the TOCTOU race between them.  In `oauth.go`'s
`Initialize()`, the background goroutine's ticker was changed from `stateMaxAge`
to `statePurgeInterval`, so expired entries are swept every 2 minutes rather
than every 10.  Tests in `server/oauth/medium_test.go` verify: injection to
exactly the cap causes the next `newState` to fail, the below-cap positive path
works, and the atomicity invariant holds (cap-1 → cap-1 insert succeeds, cap →
error returned).

