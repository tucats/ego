# WEBAUTH-M3 — `storeChallenge` mutates shared cache expiration on every call

**Affected file:** `server/server/webauthn.go:74` — `storeChallenge()`

**Description:**  
`storeChallenge` calls `caches.SetExpiration(caches.WebAuthnChallengeCache, "5m")`
on every invocation before adding the new nonce. `SetExpiration` acquires a
write lock and mutates the `Expiration` field that governs **all** entries
subsequently added to that cache. Because the duration is hardcoded to `"5m"`
the mutation is idempotent in practice, but it introduces an unnecessary
write-lock contention point on every ceremony begin request and creates a
time-of-check/time-of-use race window between the `SetExpiration` and `Add`
calls.

**Recommendation:**  
Call `SetExpiration` once at server startup alongside route registration, and
remove the call from `storeChallenge`.

**Resolution (April 2026):**  
`caches.SetExpiration(caches.WebAuthnChallengeCache, "5m")` moved from
`storeChallenge` to `defineNativeAdminHandlers` in `commands/server.go`, where
it is called once when the WebAuthn routes are registered. The redundant call
has been removed from `storeChallenge`.

