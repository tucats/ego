# WEBAUTH-M1 — No rate limiting on unauthenticated ceremony-begin endpoints

**Affected file:** `server/server/webauthn.go` — `WebAuthnLoginBeginHandler`,
`WebAuthnRegisterBeginHandler`

**Description:**  
Both begin endpoints are unauthenticated. Each successful call allocates a UUID
nonce and stores a serialized `webauthn.SessionData` value in the
`WebAuthnChallengeCache`. There is no per-IP rate limit, no global cap on the
number of pending ceremonies, and no maximum cache size. A sustained flood of
requests would continuously fill the cache, consuming server memory at a rate
bounded only by network bandwidth and the 5-minute TTL on each entry.

**Recommendation:**  
Apply a per-IP rate limit (token bucket or sliding-window counter) to both begin
endpoints, and/or enforce a maximum number of pending WebAuthn sessions at any
given time, rejecting new begin requests with 429 when the cap is reached.

**Resolution (April 2026):**  
`webAuthnBeginGuard()` added in `server/server/webauthn_limiter.go`; called at
the top of both begin handlers after `passkeyGuard`. Enforces a sliding-window
per-IP limit of 10 requests per minute and a global cap of 200 concurrent
pending ceremonies. Both limits return HTTP 429 with an audit log entry.
`clientIP()` honours `X-Forwarded-For` when the direct peer is a loopback
address. Covered by `TestIPLimiter_*` and `TestWebAuthnBeginGuard_*` tests.

