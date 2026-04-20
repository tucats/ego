# Security Issues — Authentication & Login

This document records known security weaknesses in the authentication and login
subsystem identified during a code review in April 2026. It is intended as a
living reference for future developers: each section describes the risk, the
affected code, and a concrete recommendation. A checklist at the bottom tracks
remediation progress.

---

## Critical (Login)

### LOGIN-C1 — Weak password hashing (SHA-256, no salt)

**Affected files:**

- `egostrings/gibberish.go:119` — `HashString()`
- `server/auth/validate.go:26` — call site in `ValidatePassword()`

**Description:**  
Passwords are hashed with a bare SHA-256 digest. SHA-256 is a general-purpose
hash optimized for speed, which is the opposite of what password storage
requires. Because no per-user salt is applied, all users who share the same
password produce identical hash values. An attacker who obtains the user
database can attack all accounts in parallel using precomputed rainbow tables,
and can crack typical passwords in seconds on commodity hardware.

**Recommendation:**  
Replace `HashString()` as the password-storage primitive with `bcrypt` at a
cost factor of 12 or higher (`golang.org/x/crypto/bcrypt`). Alternatively,
`argon2id` or `scrypt` are acceptable. A migration path for existing stored
hashes can be implemented by detecting the old format on login success and
re-hashing in place with the new algorithm.

---

### LOGIN-C2 — No brute-force or rate-limiting protection on the login endpoint

**Affected files:**

- `server/server/admin.go:27` — `LogonHandler()`
- `server/server/auth.go:34` — `Authenticate()`
- `server/auth/validate.go:12` — `ValidatePassword()`

**Description:**  
The `/services/admin/logon` endpoint and the Basic Auth validation path accept
an unlimited number of credential attempts with no throttling, no failed-attempt
counter, and no account lockout. An attacker can attempt passwords at full
network speed without any server-side friction.

**Recommendation:**  
Track failed authentication attempts per username (and optionally per source IP)
with a short-lived counter. Lock accounts temporarily after a threshold of
failures (e.g. 5–10), using exponential backoff before unlocking. If the
database backend is in use, the counter should survive server restarts. A
simple in-memory counter is an acceptable starting point for the file backend.

---

## High (Login)

### LOGIN-H1 — Timing attack in password comparison

**Affected file:** `server/auth/validate.go:27`

```go
ok = realPass == hashPass
```

**Description:**  
The built-in `==` operator on strings returns as soon as it finds the first
differing byte. An attacker making many requests can measure response-time
variations to determine how many leading bytes of their guess match the stored
hash — a well-known side-channel attack that can significantly narrow the
search space for an offline crack.

**Recommendation:**  
Replace the equality check with `crypto/subtle.ConstantTimeCompare`:

```go
ok = subtle.ConstantTimeCompare([]byte(realPass), []byte(hashPass)) == 1
```

---

### LOGIN-H2 — Weak key derivation in AES encryption

**Affected files:**

- `util/crypto.go` — `encrypt()` / `decrypt()` — token and DSN password encryption
- `app-cli/settings/crypto.go` — `encrypt()` / `decrypt()` — profile sidecar file encryption

**Description:**  
Both encryption layers originally derived AES keys from the passphrase using a
single MD5 computation: no salt, no iterations. MD5 produces a 128-bit output
that is hex-encoded to 32 ASCII bytes (coincidentally matching the AES-256 key
length), but the derivation is trivially fast — a GPU cluster can test billions
of candidate passphrases per second. No per-encryption salt means identical
passphrases always produce identical keys, enabling pre-computation attacks.

This is the most consequential of the two affected files because
`app-cli/settings/crypto.go` protects the highest-value secrets in the system:
`ego.server.token.key` (the AES key that signs all bearer tokens),
`ego.logon.token` (the stored bearer token), and database credentials.

**Recommendation:**  
Replace MD5 key derivation with a memory-hard KDF — Argon2id
(`golang.org/x/crypto/argon2`) — with a random per-encryption salt.
Add a version-discriminator (magic prefix) to existing ciphertext so old
data can be decrypted via the legacy MD5 path while new encryptions use the
stronger algorithm. Existing data migrates silently on the next write.

**Resolution (April 2026, stage 1):**  
`util/crypto.go` upgraded from MD5 to PBKDF2-SHA256 (100,000 iterations,
16-byte random salt). New ciphertext identified by a 4-byte magic prefix
`ÿEGO`; legacy MD5 ciphertext (no prefix) still decrypts via the existing path.

**Resolution (April 2026, stage 2):**  
Both files upgraded from their respective weak KDFs to **Argon2id**
(32 MiB memory, 2 iterations, 1 thread, 16-byte random salt) — the current
OWASP-recommended algorithm for password and key derivation.

- `util/crypto.go`: `encrypt()` now emits the `ÿEG3` magic prefix (v3).
  `decrypt()` dispatcher recognizes `ÿEG3` (Argon2id), `ÿEGO` (PBKDF2, v2),
  and the legacy no-prefix (MD5) format, so all existing tokens and DSN
  passwords continue to decrypt. New tokens and DSN passwords are written in
  v3 on the next encryption.

- `app-cli/settings/crypto.go`: `encrypt()` now emits the `ÿEG3` magic prefix.
  `decrypt()` recognizes `ÿEG3` (Argon2id) and the legacy no-prefix (MD5)
  format. Existing profile sidecar files transparently decrypt; they are
  re-encrypted in v2 on the next profile save.

---

### LOGIN-H3 — Token signing key stored in plaintext configuration

**Affected file:** `tokens/key.go:13` — `getTokenKey()`

**Description:**  
The AES key used to encrypt all bearer tokens is written to the user's settings
profile (`~/.ego/...`) in plaintext. Anyone with read access to that file can
extract the key and forge valid tokens for any username on that server instance.
The `EGO_SERVER_TOKEN_KEY` environment variable fallback also exposes the key
in the process environment, which is readable by other processes owned by the
same user on Linux (`/proc/<pid>/environ`).

**Recommendation:**  
The token key must not appear in plaintext in any configuration file. Options
include:

- Deriving the key from a master passphrase using PBKDF2 with a stored salt
  (the passphrase is never stored).
- Requiring the operator to supply the key via a secrets manager or sealed
  configuration at startup and refusing to start without it.
- At minimum, verify that the settings file is created with `0600` permissions
  and document the requirement.

**Resolution (April 2026):**  
Upon review, `ego.server.token.key` is already handled by the settings
infrastructure's "outboard encrypted config" mechanism. Sensitive keys listed in
`encryptedKeyValue` (`app-cli/settings/defs.go`) are never written to the main
profile JSON. Instead, each is written to a separate sidecar file (e.g.
`$.key`) encrypted with AES-256-GCM; the encryption passphrase is derived from
`profile.Name + profile.Salt + profile.ID`. The main profile JSON contains no
plaintext token key. This satisfies the "at minimum" requirement above for the
file-backend deployment model. The `EGO_SERVER_TOKEN_KEY` env-var path remains
a minor informational risk (see L1) but is not required for normal operation.

---

### LOGIN-H4 — Login credentials forwarded on HTTP 301 redirect to arbitrary host

**Affected file:** `app-cli/app/logon.go:167`

**Description:**  
When the logon server returns an HTTP 301 redirect, the CLI re-sends the full
credential payload (username + password in the JSON request body) to the
`Location` URL without validating that the redirect target is the same host or
a trusted authority. A network attacker, or a misconfigured authority setting,
can redirect login requests — and the credentials they carry — to an
attacker-controlled host.

**Recommendation:**  
Do not automatically re-send credentials on redirect. Either refuse to follow
3xx responses that contain a body (and ask the user to update their configured
server URL), or validate that the redirect target shares the same origin
(scheme + host + port) before resending. The existing retry loop should at
most follow redirects for unauthenticated probes (heartbeat), not for
credential POST requests.

---

## Medium (Login)

### LOGIN-M1 — HTTP downgrade when HTTPS connection fails

**Affected file:** `app-cli/app/logon.go:362` — `resolveServerName()`

**Description:**  
When resolving an unqualified server name, the CLI first tries HTTPS, then
silently falls back to plain HTTP if the HTTPS attempt fails. A network
attacker who blocks port 443 (or injects a TLS error) will cause the CLI to
retransmit the login credentials over an unencrypted connection without any
warning to the user.

**Recommendation:**  
Remove the HTTP fallback from the credential-submission path entirely. If HTTP
support is needed for development environments, require an explicit
`--insecure` or `--http` flag and print a prominent warning before sending.

---

### LOGIN-M2 — Password whitespace stripped before hashing

**Affected file:** `app-cli/app/logon.go:104`

```go
pass = strings.TrimSpace(pass)
```

**Description:**  
Leading and trailing whitespace is removed from the password before it is sent
to the server. A user who has intentionally chosen a password that begins or
ends with a space will find that their password silently changes, potentially
breaking login or allowing authentication with a truncated version of their
intended credential. It also subtly reduces the effective password space.

**Recommendation:**  
Remove the `TrimSpace` call. If legacy compatibility with passwords that were
stored after trimming is a concern, handle this explicitly in a migration step
rather than silently mangling all input going forward.

---

### LOGIN-M3 — Token cache bypasses expiry and revocation checks

**Affected file:** `server/server/auth.go:117`

```go
if userItem, found := caches.Find(caches.TokenCache, token); found {
    isAuthenticated = true
    user = data.String(userItem)
    // expiry and blacklist not rechecked
}
```

**Description:**  
When a bearer token is found in the 60-second in-memory cache, it is accepted
as valid without rechecking whether it has expired or been added to the
revocation blacklist since it was first cached. A token that expires or is
revoked can continue to authenticate requests for up to 60 seconds.

**Recommendation:**  
On a cache hit, still verify `tokens.Validate()` for expiry and blacklist
status. The cache value should store the parsed `Token` struct (including
its expiry field) rather than just the username string, so these checks can
be performed without a full re-decryption on every request.

---

### LOGIN-M4 — Quoted-password legacy format allows plaintext storage

**Affected file:** `server/auth/validate.go:22`

```go
if strings.HasPrefix(realPass, "{") && strings.HasSuffix(realPass, "}") {
    realPass = egostrings.HashString(realPass[1 : len(realPass)-1])
}
```

**Description:**  
A stored password value wrapped in `{` `}` braces is treated as a plaintext
value that is hashed at runtime instead of at storage time. This means
passwords can be deliberately (or accidentally) stored in plaintext in the
user database. Any user record whose stored password field begins and ends
with braces is vulnerable to having its password derived from the inner value,
which could enable authentication bypass in unexpected edge cases.

**Recommendation:**  
Treat this as a migration-only escape hatch. Add a guard so that the
`{plaintext}` form can only be consumed once — on the first successful login,
re-hash the value with the production algorithm and store the result, removing
the braces. Log a warning when the legacy format is encountered. Once all
known passwords have been migrated, remove the special-case logic entirely.

---

## Low / Informational (Login)

### LOGIN-L1 — Password supplied via environment variable

**Affected file:** `app-cli/app/logon.go:38` — `EgoPasswordEnv`

Environment variables are visible to all processes owned by the same user and
are inherited by child processes. Using `EGO_PASSWORD` to supply credentials is
a common CI convenience but should be noted in any threat model. Consider
supporting a credentials file with `0600` permissions or a secrets-manager
integration as a more secure alternative for automated contexts.

**Resolution (April 2026):**  
`app-cli/app/logon.go` now checks `os.Getenv(defs.EgoPasswordEnv)` after
reading the password option. When the variable is set, `ui.Say("logon.password.env")`
emits a visible warning to the user's console regardless of log level, and
`os.Unsetenv` clears the variable immediately so child processes do not inherit
the credential. Localized strings added to all three language files.

---

### LOGIN-L2 — `InsecureSkipVerify` available without prominent warning

TLS certificate verification can be disabled for self-signed certificates.
In development this is acceptable, but the flag should trigger a visible
warning in production-like configurations and should not be silently inherited
by default from a stored profile setting.

**Resolution (April 2026):**  
Two always-visible warning paths added using `ui.Say("rest.tls.insecure")`:
one in `runtime/rest/exchange.go` when the `ego.runtime.insecure.client`
profile setting activates insecure mode, and one in `runtime/rest/client.go`
when the `EGO_INSECURE_CLIENT` environment variable triggers it. For the
Ego-program `rest.Open({verify:false})` path in `runtime/rest/methods.go`, a
REST-logger entry is added (the disable is deliberate user code in that context,
not silent ambient configuration). Localized warning strings added to all three
language files under the key `rest.tls.insecure`.

---

### LOGIN-L3 — Returned token expiration recalculated independently of token contents

**Affected file:** `server/server/admin.go:99`

The expiration timestamp returned in the logon response body is computed from
the server's current max-expiration setting at response time, independently of
the expiry embedded inside the encrypted token. If the server's configured
maximum token duration changes between token issuance and token use, the
advisory expiration seen by the client and the enforced expiry inside the token
can diverge. This does not affect security directly but can cause confusing
"token expired" errors before the displayed expiry has passed.

**Resolution (April 2026):**  
`LogonHandler` in `server/server/admin.go` now calls `tokens.Unwrap` on the
freshly-minted token string immediately after creating it. `response.Expiration`
is set from `t.Expires.Format(time.UnixDate)` — the value baked into the token
itself — and the previous independent duration re-calculation block has been
removed. The `tokens.Unwrap` call also surfaces the `TokenID` needed for
`response.ID`, consolidating two previously separate concerns into one call.

---

---

## Security Issues — WebAuthn / Passkeys

This section records security weaknesses in the WebAuthn passkey subsystem
identified during a code review in April 2026. Issues are rated using the same
severity scale as the authentication section above.

---

### High (WebAuthn)

#### WEBAUTH-H1 — `allow.passkeys` setting not enforced in ceremony handlers

**Affected file:** `server/server/webauthn.go` — `WebAuthnLoginBeginHandler`,
`WebAuthnLoginFinishHandler`, `WebAuthnRegisterBeginHandler`,
`WebAuthnRegisterFinishHandler`, `WebAuthnClearPasskeysHandler`

**Description:**  
The `ego.server.allow.passkeys` setting was only checked in
`WebAuthnConfigHandler`, which returns the feature-flag value to the dashboard.
All five ceremony handlers ignored the setting entirely. A caller who bypassed
the dashboard UI (e.g. `curl`) could drive the full passkey registration and
login ceremonies even when an operator had disabled passkeys on the server.
The UI gate provided security by obscurity only.

**Recommendation:**  
Each ceremony handler must check the setting at entry and return 404 before
performing any work when passkeys are disabled. Extract the check into a shared
`passkeyGuard()` helper to ensure consistent enforcement.

**Resolution (April 2026):**  
`passkeyGuard()` added to `server/server/webauthn.go`; called at the top of all
five handlers. Returns HTTP 404 with an `auth.webauthn.disabled` audit log entry
when passkeys are disabled. Covered by `TestPasskeyGuard_*` tests.

---

#### WEBAUTH-H2 — Authenticator clone warning not checked after login

**Affected file:** `server/server/webauthn.go:302` — `WebAuthnLoginFinishHandler`

**Description:**  
The WebAuthn specification (§6.1, step 21) requires servers to detect when an
authenticator's sign counter does not advance between uses, which is a strong
indicator that the credential has been cloned and is being replayed. The
`go-webauthn` library surfaces this condition as
`credential.Authenticator.CloneWarning == true` on the returned
`*webauthn.Credential`. The code was inspecting neither the returned credential
nor this flag, so a cloned passkey would authenticate successfully.

**Recommendation:**  
After `FinishDiscoverableLogin` returns without error, inspect
`credential.Authenticator.CloneWarning`. If `true`, reject the login with 401
and log an audit event naming the affected user.

**Resolution (April 2026):**  
Clone check added immediately after the `FinishDiscoverableLogin` call in
`WebAuthnLoginFinishHandler`. A `true` CloneWarning rejects the login with 401
and emits `auth.webauthn.clone.warning` to the AUTH audit log. Localized strings
added to all three language files.

---

### Medium (WebAuthn)

#### WEBAUTH-M1 — No rate limiting on unauthenticated ceremony-begin endpoints

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

---

#### WEBAUTH-M2 — RPID derived from user-controlled `Host` header

**Affected file:** `server/auth/webauthn.go:72` — `NewWebAuthnForRequest()`

**Description:**  
When `ego.server.webauthn.rpid` is not configured, the RPID and allowed origin
are derived directly from the `r.Host` header, which is caller-supplied. Behind
a misconfigured reverse proxy an attacker can set an arbitrary `Host` value,
causing the server to issue challenges bound to an attacker-chosen RPID. The
real users' passkeys will not satisfy such challenges, but the attack can
generate junk ceremony state and is a defense-in-depth gap.

**Recommendation:**  
Document that `ego.server.webauthn.rpid` **must** be set in any
internet-facing or production deployment. Add a startup warning when the setting
is absent and the server is not bound to `localhost`. The auto-derive path
should be treated as a local-development convenience only.

**Resolution (April 2026):**  
A startup warning is emitted via the `server.webauthn.no.rpid` SERVER log key
in `defineNativeAdminHandlers` (`commands/server.go`) when
`ego.server.allow.passkeys` is true and `ego.server.webauthn.rpid` is empty.
Localized strings added to all three language files.

---

#### WEBAUTH-M3 — `storeChallenge` mutates shared cache expiration on every call

**Affected file:** `server/server/webauthn.go:74` — `storeChallenge()`

**Description:**  
`storeChallenge` calls `caches.SetExpiration(caches.WebAuthnChallengeCache, "5m")`
on every invocation before adding the new nonce. `SetExpiration` acquires a
write lock and mutates the `Expiration` field that governs **all** entries
subsequently added to that cache. Because the duration is hardcoded to `"5m"`
the mutation is idempotent in practice, but it introduces an unnecessary
write-lock contention point on every ceremony begin request and creates a time-of-check/time-of-use race window between the `SetExpiration` and `Add` calls.

**Recommendation:**  
Call `SetExpiration` once at server startup alongside route registration, and
remove the call from `storeChallenge`.

**Resolution (April 2026):**  
`caches.SetExpiration(caches.WebAuthnChallengeCache, "5m")` moved from
`storeChallenge` to `defineNativeAdminHandlers` in `commands/server.go`, where
it is called once when the WebAuthn routes are registered. The redundant call
has been removed from `storeChallenge`.

---

### Low / Informational (WebAuthn)

#### WEBAUTH-L1 — `Secure` flag absent on challenge cookie

**Affected file:** `server/server/webauthn.go:51` — `challengeCookie()`

**Description:**  
The nonce cookie used to correlate the browser with the server's challenge cache
is `HttpOnly` and `SameSite: Strict`, but the `Secure` attribute is not set.
In any configuration where the server accepts plain HTTP (before an HTTPS
redirect, or in a development setup), the nonce could be transmitted in
cleartext. Browsers enforce HTTPS for WebAuthn themselves, which limits
practical exposure in the field, but the cookie hardening is incomplete.

**Resolution:**  
`challengeCookie` now accepts a `secure bool` parameter. The `isSecureRequest(r)`
helper returns `true` when `r.TLS != nil` or `X-Forwarded-Proto: https` is set.
All four call sites pass `isSecureRequest(r)`.

---

#### WEBAUTH-L2 — Cache item expiration only refreshed when CACHE logging is active

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

---

#### WEBAUTH-L3 — No user notification when passkeys are cleared by an administrator

**Affected file:** `server/server/webauthn.go` — `WebAuthnClearPasskeysHandler`

**Description:**  
An administrator can silently remove all passkeys for any user account. The
operation is audit-logged server-side, but the affected user receives no
in-band notification. A compromised admin account could degrade every user's
authentication to password-only without any visible indication to those users.

**Resolution:**  
`WebAuthnClearPasskeysHandler` now emits a `SERVER`-logger entry using the new
`server.webauthn.admin.cleared.passkeys` key when the actor differs from the
target user. The SERVER log level is always shown in the dashboard Log tab,
making the action visible without requiring elevated log settings.

---

## Security Issues — HTTP Server

This section covers vulnerabilities in the HTTP request/response pipeline as
implemented in `server/server/serve.go`, `server/server/router.go`, and the
server-startup code in `commands/server.go`. These issues are independent of the
authentication and WebAuthn concerns documented above.

---

### High (HTTP Server)

#### HTTP-H1 — No request body size limit

**Affected file:** `server/server/serve.go:313`

```go
session.Body, _ = io.ReadAll(r.Body)
```

**Description:**  
Every non-lightweight request has its body read into memory with a bare
`io.ReadAll`. There is no call to `http.MaxBytesReader` and no size check
before the read. An attacker — authenticated or not — can send a POST or PUT
request with a body of arbitrary size (limited only by their bandwidth and the
server's available RAM). Because the read happens unconditionally before the
handler is invoked, even endpoints that ignore the body will fully consume it.
A single large request can cause the Go garbage collector to thrash; a flood of
them can exhaust virtual memory and crash the process.

**Recommendation:**  
Wrap the request body with `http.MaxBytesReader` before calling `io.ReadAll`.
Choose a generous-but-bounded limit appropriate to the largest legitimate
payload (e.g., 32 MiB for the Ego server's use cases, configurable via a
setting):

```go
const maxBodyBytes = 32 << 20  // 32 MiB

r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
session.Body, _ = io.ReadAll(r.Body)
```

`http.MaxBytesReader` returns a `*http.MaxBytesError` when the limit is
exceeded, which `io.ReadAll` surfaces as a non-nil error. Check the error and
return 413 Request Entity Too Large.

---

#### HTTP-H2 — No timeouts on the HTTP server — Slowloris vulnerability

**Affected files:**

- `commands/server.go:250` — plain HTTP path: `http.ListenAndServe(addr, router)`
- `commands/server.go:506` — TLS path: `http.ListenAndServeTLS(addr, certFile, keyFile, router)`

**Description:**  
Both server start paths use the convenience functions `http.ListenAndServe` and
`http.ListenAndServeTLS`, which create an `http.Server` with all timeout fields
left at their zero value — meaning no timeout at all. This makes the server
vulnerable to the Slowloris attack: an attacker opens many connections and
sends HTTP request headers one byte at a time, never completing them. Each
such connection holds a Go goroutine and a file descriptor open indefinitely.
With enough connections (default Linux limit is typically 1024 open file
descriptors per process), the server stops accepting new legitimate requests.
No authentication is required — the attack happens before the request headers
are complete.

**Recommendation:**  
Replace the convenience functions with an explicit `http.Server` that sets
all four timeout fields. Recommended starting values:

```go
srv := &http.Server{
    Addr:              addr,
    Handler:           router,
    ReadHeaderTimeout: 10 * time.Second,  // time to receive all headers
    ReadTimeout:       60 * time.Second,  // time to receive the full request
    WriteTimeout:      120 * time.Second, // time to send the full response
    IdleTimeout:       120 * time.Second, // keep-alive idle before closing
}
err = srv.ListenAndServeTLS(certFile, keyFile)
```

`ReadHeaderTimeout` is the most critical: it directly stops the Slowloris
pattern by closing connections that do not finish sending headers within the
window. The values above are reasonable defaults; consider making them
configurable via server settings for environments with large response bodies
(e.g., log retrieval).

---

### Medium (HTTP Server)

#### HTTP-M1 — Security response headers not set

**Affected file:** `server/server/serve.go` — `ServeHTTP()`

**Description:**  
The server never adds any of the standard browser-security response headers.
While most Ego server clients are CLI tools or the dashboard SPA rather than
general browsers, the dashboard is a browser application and omitting these
headers leaves it exposed to well-known classes of attack:

| Missing header | Risk if absent |
| :--- | :--- |
| `Content-Security-Policy` | XSS via injected scripts in the dashboard |
| `X-Content-Type-Options` (no-sniff) | MIME-sniffing attacks on API responses |
| `X-Frame-Options: DENY` | Click-jacking via embedding the dashboard in an iframe |
| `Strict-Transport-Security` | Browser allows downgrade to HTTP after first HTTPS visit |
| `Referrer-Policy` | Auth tokens in URL leaked via the HTTP referrer request header |

**Recommendation:**  
Add a thin middleware layer (or add directly to `ServeHTTP` before calling the
handler) that sets the defensive headers on every response:

```go
w.Header().Set("X-Content-Type-Options", "nosniff")
w.Header().Set("X-Frame-Options", "DENY")
w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
if r.TLS != nil {
    w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
}
```

A `Content-Security-Policy` appropriate for the dashboard requires more
thought (it must whitelist the sources used by the dashboard's JavaScript and
CSS), but at minimum a restrictive default of `default-src 'self'` should be
set and relaxed only for the dashboard routes that need it.

Due to dashboard problems, current CSP settings are as follows. The unsafe-inline
was added because dashboard was being blocked from itself.

- default-src 'self'
- script-src 'self' 'unsafe-inline'
- style-src 'self' 'unsafe-inline'
- object-src 'none'
- base-uri 'self'

---

#### HTTP-M2 — Server UUID and session counter disclosed on every response

**Affected file:** `server/server/serve.go:65`

```go
w.Header()[defs.EgoServerInstanceHeader] = []string{
    fmt.Sprintf("%s:%d", defs.InstanceID, sessionID),
}
```

**Description:**  
Every non-lightweight response carries the `X-Ego-Server` header whose value
is `<UUID>:<sessionID>`. This discloses two pieces of information:

1. **Persistent server UUID** — `defs.InstanceID` is stable for the life of the
   server process. An attacker learns a fingerprint that lets them confirm they
   are talking to the same server instance across requests, enumerate multiple
   instances in a cluster, and correlate responses in log analysis.

2. **Monotonically increasing session counter** — `sessionID` is a global
   sequence number that increments for every request. An attacker can measure
   the exact rate of legitimate traffic, detect low-activity windows optimal
   for an attack, and infer whether a request they injected was processed by
   comparing counter values before and after.

Neither piece of information is needed by well-behaved clients; the dashboard
reads it only for display purposes. The same UUID is already in the
`server_info` JSON body for clients that genuinely need it.

**Recommendation:**  
Remove the header from production responses, or make it opt-in for clients
that need it (e.g. behind an `X-Ego-Debug` request header only recognized
when debug logging is active). At minimum, omit the session counter from the
header value so the UUID alone is disclosed — it is already available in the
response body for clients that need it.

---

#### HTTP-M3 — Redirect server (port 80) has no timeouts

**Affected file:** `commands/server.go:705` — `redirectToHTTPS()`

```go
httpSrv := http.Server{
    Addr:    httpAddr,
    Handler: http.HandlerFunc(...),
}
```

**Description:**  
The plain-HTTP listener created to redirect traffic from port 80 to HTTPS
suffers the same timeout omission as the main server (HTTP-H2). Because it
must remain reachable from the public internet to redirect HTTP clients, it is
the most exposed component, yet it has zero protection against slow-reading
attackers.

**Recommendation:**  
Apply the same `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, and
`IdleTimeout` values to `httpSrv` as recommended in HTTP-H2. For a redirect-
only server, even more aggressive timeouts are appropriate (e.g.,
`ReadHeaderTimeout: 5s`), since legitimate redirect clients will complete their
headers in milliseconds.

---

### Low / Informational (HTTP Server)

#### HTTP-L1 — User-supplied URL path reflected verbatim in error messages

**Affected file:** `server/server/serve.go:79`

```go
msg = "endpoint " + r.URL.Path + " not found"
```

**Description:**  
When a route is not found, the raw URL path from the request is concatenated
directly into the error message string that is returned to the client (and
written to the server log). This is not a direct injection risk for JSON-
consuming API clients, but it has two minor consequences:

1. **Information disclosure** — The exact URL string the attacker sent is
   reflected back, confirming what path patterns do and do not exist on the
   server. This makes reconnaissance easier.
2. **Log injection** — If the URL path contains newline characters or log-
   format control sequences, they appear verbatim in the structured SERVER log
   entry, potentially corrupting log output or confusing log parsers.

**Recommendation:**  
Return a generic message to the client (`"not found"`) and include the raw path
only in the server-side log:

```go
util.ErrorResponse(w, sessionID, "not found", status)
ui.Log(ui.ServerLogger, "server.route.error", ui.A{
    ...
    "path": r.URL.Path,  // path stays in the log, not in the response
})
```

---

#### HTTP-L2 — Request body read after permission check already set a failure status

**Affected file:** `server/server/serve.go:302–318`

**Description:**  
When the `mustBeAdmin` check fails (the user is authenticated but lacks admin
privileges), the code sets `status = http.StatusForbidden` but does **not**
`return`. Execution falls through to the unconditional `io.ReadAll(r.Body)` on
line 313, which reads the full request body into memory before the handler
(correctly) declines to run. The same pattern applies to the
`mustAuthenticate + canAuthenticate` branch that sets 401. In both cases a
non-privileged caller can force the server to allocate memory for whatever body
they attach to a privileged endpoint. While the individual impact is low, it
directly compounds HTTP-H1 (no body size limit): a stream of 403-destined
requests with large bodies can exhaust memory without ever triggering the
handler path.

**Recommendation:**  
Add an early `return` after each auth/permission failure that sets a non-OK
status, so the body is never read for requests that are going to be rejected:

```go
} else if route.mustBeAdmin && !session.Admin {
    ...
    util.ErrorResponse(w, session.ID, "not authorized", http.StatusForbidden)
    return  // ← add this
}
```

---

---

## Security Issues — Tables Endpoint

This section records security weaknesses in the `/tables/` and `/dsns/` endpoint
handlers as implemented in `server/tables/`, identified during a code review in
April 2026.

---

### Critical (Tables)

#### TABLES-C1 — `CommitHandler` inverted guard panics on every valid commit

**Affected file:** `server/tables/transactions.go:207` — `CommitHandler()`

```go
parameters := session.Parameters[defs.TransactionIDParameterName]
if len(parameters) != 0 {          // ← should be != 1
    return util.ErrorResponse(...)  // rejects requests WITH a transaction ID
}
id := data.String(parameters[0])   // panics: index 0 on empty slice
```

**Description:**  
The guard condition is inverted relative to `RollbackHandler` (line 170, which
correctly uses `!= 1`). The result is that any request that arrives *with* a
valid transaction ID parameter — i.e., every legitimate commit — is immediately
rejected with "missing transaction ID". Any request that arrives *without* a
transaction ID parameter — i.e., every malformed commit — passes the guard and
then panics on line 212 (`parameters[0]` on an empty slice), crashing the
goroutine handling that connection.

**Recommendation:**  
Change `!= 0` to `!= 1` on line 207, matching `RollbackHandler`.

---

### High (Tables)

#### TABLES-H1 — SQL injection via raw username in table-list query

**Affected file:** `server/tables/list.go:69` — `listTables()`

```go
schema := session.User
q := strings.ReplaceAll(tablesListQuery, "{{schema}}", schema)
// tablesListQuery = `... WHERE table_schema = '{{schema}}' ORDER BY ...`
```

**Description:**  
The authenticated user's name is substituted directly into a SQL string using
`strings.ReplaceAll`, bypassing the `parsing.QueryParameters` / `SQLEscape`
pipeline that all other query parameters go through. The value lands inside a
single-quoted SQL literal, but `SQLEscape` is never called. A username whose
middle characters include a single quote (e.g. `O'Reilly`) breaks the literal
and allows injection:

```sql
WHERE table_schema = 'O'Reilly' ORDER BY table_name
```

A more deliberately crafted name (`x' UNION SELECT username,passwd FROM
pg_shadow--`) could exfiltrate sensitive data from the database.

**Recommendation:**  
Route the schema substitution through `parsing.QueryParameters`, which calls
`SQLEscape` on every value. Alternatively, use a parameterized query where the
driver handles quoting: `db.Query("... WHERE table_schema = $1 ...", schema)`.

---

#### TABLES-H2 — Inverted authorization filter in `ListTables` leaks table names

**Affected file:** `server/tables/list.go:136` — `getTableNames()`

```go
if !db.Session.Admin && Authorized(db.Session, db.Session.User, name, defs.TableReadPermission) {
    continue  // skips tables the user IS authorized to read
}
```

**Description:**  
The `!` before `Authorized` is missing. `Authorized` returns `true` when the
user has the requested permission. The current code therefore skips (hides)
every table the user *is* permitted to read, and exposes every table they are
*not* permitted to read.

For non-secured DSNs (the common case) `Authorized` always returns `true`, so
the `continue` fires for every table and non-admin users see an empty list —
a functional denial of service. For secured DSNs the effect is reversed access
control: tables a non-admin user may read are hidden from them, while tables
they have no business seeing are visible.

**Recommendation:**  
Add the negation: `if !db.Session.Admin && !Authorized(...)`.

---

### Medium (Tables)

#### TABLES-M1 — SQL injection in `DeleteTable` via un-parameterized `DROP TABLE`

**Affected file:** `server/tables/tables.go:327` — `DeleteTable()`

```go
if dsnName != "" {
    tableName = table                // table = data.String(session.URLParts["table"])
    q = "DROP TABLE " + tableName   // raw concatenation
}
```

**Description:**  
When a DSN name is present in the request, the table-deletion query is
constructed by concatenating the URL path parameter directly into a SQL
string, without quoting or escaping. An authenticated caller who can reach
the delete-table endpoint can supply a table name such as
`users; DROP TABLE admin; --` to execute arbitrary SQL on the DSN's database.
The code path that uses `parsing.QueryParameters` (lines 316–318) is bypassed
entirely in the DSN case.

**Recommendation:**  
Use double-quote escaping consistent with the non-DSN path:
`q = "DROP TABLE \"" + tableName + "\""`. Better still, route through
`parsing.QueryParameters` regardless of whether a DSN is present.

---

#### TABLES-M2 — Row ID concatenated un-parameterized into `UPDATE` WHERE clause

**Affected file:** `server/tables/parsingAbstract.go:77` — `formAbstractUpdateQuery()`

```go
where = "WHERE " + defs.RowIDName + " = '" + idString + "'"
```

**Description:**  
The row ID value taken from the update payload is placed inside a SQL WHERE
clause using string concatenation with single-quote delimiters. While the rest
of the UPDATE statement uses `$N` parameterized placeholders (lines 62–63),
the row-ID filter reverts to direct embedding. A caller who can control the
row ID value (e.g. from the JSON body of a PATCH request) and supplies
`' OR '1'='1` causes the UPDATE to affect every row in the table.

**Recommendation:**  
Append a numbered parameter for the row ID instead of interpolating it:

```go
where = fmt.Sprintf("WHERE %s = $%d", defs.RowIDName, filterCount+1)
// pass idString as the corresponding argument to db.Exec
```

---

#### TABLES-M3 — Wrong permission constant in `DeleteTable` allows any authenticated user to delete tables

**Affected file:** `server/tables/tables.go:312` — `DeleteTable()`

```go
if !isAdmin && dsnName == "" && !Authorized(session, user, tableName, defs.AdminAgent) {
    return util.ErrorResponse(..., "User does not have read permission", ...)
}
```

**Description:**  
Two bugs combine here. First, `defs.AdminAgent` has the value `"admin"` — a
string constant identifying an agent type, not a table-permission name. The
`Authorized` switch statement has no case for `"admin"`, so the per-operation
loop runs without ever setting `auth = false`, and `Authorized` returns `true`
for any authenticated user. The guard `!Authorized(...)` is therefore always
`false`, meaning no non-admin user is ever blocked from deleting a table they
do not own. Second, the error message says "read permission" when the intent
is an admin/delete permission check.

**Recommendation:**  
Replace `defs.AdminAgent` with `defs.TableAdminPermission` (= `"ego.table.admin"`)
and update the error message accordingly.

---

#### TABLES-M4 — No server-side cap on query result size

**Affected file:** `server/tables/parsing/generators.go:748` — `PagingClauses()`

```go
if limit != 0 {
    result.WriteString(" LIMIT ")
    result.WriteString(strconv.Itoa(limit))
}
// No maximum enforced; if limit == 0 (absent or zero), no LIMIT clause added.
```

**Description:**  
When no `?limit=` query parameter is supplied, or when it is supplied as `0`,
no `LIMIT` clause is appended to the generated SQL. An authenticated caller
can retrieve an entire table — potentially millions of rows — in a single
request. The rows are buffered in the server process before being marshalled
to JSON and written to the response, making this an effective memory-exhaustion
DoS against the server.

**Recommendation:**  
Enforce a server-side maximum: if `limit <= 0 || limit > maxRowLimit`, set
`limit = defaultRowLimit` (e.g. 1000). Expose the maximum as a configurable
setting. Document the behavior so callers can paginate deliberately.

---

### Low / Informational (Tables)

#### TABLES-L1 — Raw database error messages returned to clients

**Affected files (representative):**

- `server/tables/list.go:51,56`
- `server/tables/transactions.go:225`
- `server/tables/tables.go:320`

```go
msg := fmt.Sprintf("Database list error, %v", err)
util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
```

**Description:**  
Database driver errors — which include table names, column names, constraint
names, SQL syntax fragments, and internal driver details — are forwarded
verbatim to the HTTP response body. An attacker can exploit error responses to
enumerate schema structure, discover column and constraint names without having
read access, and tailor further injection attempts to the precise SQL dialect
in use.

**Recommendation:**  
Log the full error at `ui.DBLogger` and return a generic message
(`"database operation failed"`) to the caller. Reserve detailed messages for
the server log where access is controlled.

---

#### TABLES-L2 — SQLite `PRAGMA` statements use unquoted table and index names

**Affected file:** `server/tables/describe.go:244,276,307`

```go
q := fmt.Sprintf("PRAGMA index_list(%s)", tableName)
q := fmt.Sprintf("PRAGMA index_info(%s)", index)
q  = fmt.Sprintf("PRAGMA table_info(%s)", tableName)
```

**Description:**  
SQLite PRAGMA arguments are not parameterizable via `database/sql`, but names
with spaces or special characters still need to be quoted. The table names
passed here come from the server's own schema metadata (a prior `sqlite_schema`
query), so the practical risk is low — an attacker would need to have already
created a table with a malicious name. Nevertheless, the pattern violates the
principle of always quoting identifiers and would allow a name such as
`my table` (with a space) to silently truncate the PRAGMA to `PRAGMA
index_list(my)`.

**Recommendation:**  
Wrap each name in backticks or double-quotes:

```go
q := fmt.Sprintf("PRAGMA index_list(\"%s\")", tableName)
```

---

## Security Issues — Asset Handler

This section records security weaknesses in the `/assets/` endpoint as implemented
in `server/assets/handler.go`, identified during a code review in April 2026.

---

### High (Assets)

#### ASSET-H1 — DoS via open-ended `Range` header

**Affected file:** `server/assets/handler.go:296` — `readAssetRange()`

**Description:**  
A request carrying `Range: bytes=N-` (open-ended range, no explicit end byte)
leaves the `end` variable at its sentinel value `EndOfData = math.MaxInt64`.
Because `start != StartOfData`, the Loader skips the cache path and calls
`readAssetRange`. Inside that function, `size := end - start` evaluates to
approximately 9.2 EB, and the subsequent `make([]byte, size)` attempts to
allocate that many bytes. The Go runtime raises an out-of-memory condition
before the allocation completes, which — if not recovered — terminates the
server process. A single unauthenticated GET request with a valid asset path
and an open-ended Range header is sufficient to trigger this.

**Recommendation:**  
After calling `os.Stat` to obtain the true file size, clamp `end` to
`totalSize - 1` whenever it equals `EndOfData` or exceeds the file size.
This is the correct RFC 7233 interpretation of an open-ended range.

**Resolution (April 2026):**  
Clamping added in `readAssetRange` immediately after `totalSize = info.Size()`.
`size` is now computed as `end - start + 1` (inclusive, per RFC 7233). The
`make` call is therefore bounded by the actual file size.

---

### Medium (Assets)

#### ASSET-M1 — Wrong `Content-Range` header for open-ended ranges

**Affected file:** `server/assets/handler.go:176` — `AssetsHandler()`

**Description:**  
When a client sends `Range: bytes=N-`, the `end` variable in `AssetsHandler`
remains `EndOfData` (= `math.MaxInt64`) after parsing, because no explicit end
byte was specified. The handler then wrote this unmodified value directly into
the `Content-Range` response header: `bytes N-9223372036854775807/1000`. This
violates RFC 7233 §4.2, which requires the last-byte-pos in the Content-Range
header to reflect the actual last byte sent (i.e. `totalSize - 1`). Clients
that strictly parse the Content-Range value may reject or mishandle the
response.

**Recommendation:**  
After `Loader` returns `totalSize`, clamp `end` to `totalSize - 1` before
formatting the Content-Range header.

**Resolution (April 2026):**  
A `reportEnd` local variable is computed from `end`, clamped to `totalSize - 1`
when `end == EndOfData || end >= totalSize`, and used in the `Content-Range`
header format string. The handler's own `end` variable is not mutated.

---

#### ASSET-M2 — Error response discloses server filesystem paths

**Affected file:** `server/assets/handler.go:131` — `AssetsHandler()`

**Description:**  
When `Loader` returned an error (file not found, permission denied, etc.), the
handler embedded the raw OS error string — which contains the absolute
filesystem path — directly in the JSON response body:
`{"err": "open /home/tom/ego/lib/foo.txt: no such file or directory"}`. A
partial `strings.ReplaceAll` only stripped the `services` subdirectory; all
other paths were exposed verbatim. An unauthenticated caller could use this to
map the server's directory layout, confirm the existence of files, and discover
the configured `EGO_PATH`.

**Recommendation:**  
Return a fixed generic message to the caller and keep full error detail in the
server log only.

**Resolution (April 2026):**  
The error branch in `AssetsHandler` now writes the literal string
`{"err": "asset not found"}` to the response for all load failures. The
original error (including the real path) continues to be written to the
`AssetLogger` via the existing `asset.load.error` log key.

---

### Low / Informational (Assets)

#### ASSET-L1 — Double `os.ReadFile` call in `readAssetFile`

**Affected file:** `server/assets/handler.go:330,356` — `readAssetFile()`

**Description:**  
`os.ReadFile(fn)` was called twice: once at line 330 (result captured in `data`
and `err`), then again at line 356 as the function's return value. The first
result was used only for logging and then discarded; the second read was what
actually propagated to the caller. If the file was modified or deleted between
the two reads, the caller received different content (or an error) than what
was logged. In addition, it doubled the I/O cost of every uncached asset load.

**Recommendation:**  
Return the `data` and `err` captured by the first read.

**Resolution (April 2026):**  
`return os.ReadFile(fn)` at line 356 replaced with `return data, err`.

---

#### ASSET-L2 — Path normalization uses fragile string removal instead of `filepath.Clean`

**Affected file:** `server/assets/handler.go:359` — `normalizeAssetPath()`

**Description:**  
The function removed `..` components by calling `strings.ReplaceAll(path, "..", "")`
twice — once on the relative path and once on the fully-joined path. While no
active bypass was demonstrated against this specific code, naive string removal
is a well-known anti-pattern for path traversal defense: variations such as
`....//` survive the replacement and produce unexpected results after
`filepath.Join` normalizes double-slashes. The `AssetsHandler` check at line 69
only guards against `/../` mid-path and would not catch a traversal that reached
`normalizeAssetPath` directly (e.g. via the exported `Loader` function called
from dashboard handlers).

**Recommendation:**  
Use `filepath.Clean(filepath.Join(root, path))` for canonical resolution, then
verify the result is still inside `root` with a `strings.HasPrefix` confinement
check. Return a guaranteed-nonexistent path on confinement failure so the
caller handles it uniformly as a 404.

**Resolution (April 2026):**  
`normalizeAssetPath` rewritten to: (1) compute `root` as before, (2) call
`filepath.Clean(filepath.Join(root, path))` for one-pass canonical resolution,
(3) verify the result starts with `root + string(filepath.Separator)`, and
(4) return `filepath.Join(root, "__invalid__")` on confinement failure.
The loop that stripped leading dots/slashes and both `strings.ReplaceAll`
calls have been removed.

---

## Security Issues — Profile Encryption

This section records security weaknesses in the profile encryption subsystem
implemented in `app-cli/settings/crypto.go`. Profile encryption protects
sensitive configuration values — bearer tokens, database credentials, the
server token-signing key — that are stored in sidecar files alongside the main
profile JSON or in the settings database.

---

### High (Profile)

#### PROFILE-H1 — MD5 used to derive the AES encryption key

**Affected file:** `app-cli/settings/crypto.go` — `Hash()` / `encrypt()` / `decrypt()`

```go
// old code
block, _ := aes.NewCipher([]byte(Hash(passphrase)))
// Hash() returns hex.EncodeToString(md5.Sum(passphrase))
```

**Description:**  
The `Hash()` function runs the passphrase through MD5 and hex-encodes the
result to produce a 32-character string used as the raw AES-256 key. MD5 has
been cryptographically broken since 1996: collision attacks are trivially
feasible on commodity hardware, and pre-image resistance is significantly
weakened. While the 32-byte key length technically satisfies AES-256's
requirement, the key material itself carries only the entropy of an MD5
digest. An attacker who obtains a sidecar file (e.g. `$.token`, `$.key`,
`$.cred2`) can attempt an offline dictionary or brute-force attack using MD5
at billions of hashes per second on a consumer GPU, orders of magnitude faster
than would be possible with a modern KDF.

The passphrase itself is `profile.Name + profile.Salt + profile.ID`, which
provides some variable entropy, but the MD5 layer strips away any security
margin that salt would otherwise add.

**Recommendation:**  
Replace the `Hash()`-based key derivation with `sha256.Sum256([]byte(passphrase))`
which produces 32 bytes directly without the MD5 weakness and without the
double-encoding through hex. For a further improvement, adopt PBKDF2 or
HKDF with the profile salt as the KDF salt and a suitable iteration count,
though this requires a format change to store the per-value salt.

To preserve backward compatibility with existing sidecar files, prefix newly
encrypted output with a version tag (e.g. `"v2:"`) and detect the absence of
the tag in `Decrypt()` to route to the legacy decryption path.

**Resolution (April 2026):**  
`encrypt()` and `decrypt()` in `app-cli/settings/crypto.go` now derive the
AES-256 key via `deriveKey()`, which calls `sha256.Sum256([]byte(passphrase))`
and returns the 32-byte digest directly. All new ciphertext produced by
`Encrypt()` is prefixed with `"v2:"` before base64 encoding. `Decrypt()`
inspects the prefix: a `"v2:"` prefix routes to the SHA-256 path; the absence
of a prefix routes to `decryptLegacy()`, which preserves the original MD5-hex
key derivation for existing stored values. `decryptLegacy()` is clearly marked
for future removal once all stored values have been re-encrypted.

---

### Medium (Profile)

#### PROFILE-M1 — Legacy ciphertext not re-encrypted on successful read

**Affected files:** `app-cli/settings/files.go:303`, `app-cli/settings/databases.go:260`

**Description:**  
When `Decrypt()` successfully decodes a value that has no `"v2:"` prefix, the
result is returned and stored in the in-memory configuration, but the
underlying sidecar file (or database field) retains the old MD5-encrypted
ciphertext. As long as legacy values are never written back — for example, if
the setting is never explicitly changed — they remain permanently protected
only by the weaker MD5 key derivation. A long-running server that never
touches certain settings (the token key, for instance) may never trigger a
write that would upgrade those values.

**Recommendation:**  
After a successful `decryptLegacy()` call in the `Decrypt()` routing, mark
the configuration as dirty so that the next profile save re-encrypts the value
with the new SHA-256 scheme. Alternatively, perform an eager re-encrypt-and-
save immediately after the read. Either approach ensures that legacy ciphertext
is upgraded within one profile-read cycle.

**Resolution (April 2026):**  
Both read paths now call `NeedsNewHash()` on the raw ciphertext immediately
after a successful `Decrypt()`. When it returns true, the plaintext is
re-encrypted with `Encrypt()` (SHA-256 / v2 scheme) and written back to the
same storage location before the function returns:

- **File path** (`app-cli/settings/files.go` — `readOutboardConfigFiles()`):
  the sidecar file is overwritten with the new ciphertext via `os.WriteFile`.
- **Database path** (`app-cli/settings/databases.go` — `Load()`): upgraded
  items are collected during the row scan and written back with `UPDATE`
  statements after the cursor is closed.

Both paths log `config.reencrypted` (the i18n key) on success. After this change, legacy
MD5-encrypted values are upgraded to SHA-256 on the first read, with no
manual migration step required.

---

## Remediation Checklist

Use this checklist to track progress as issues are resolved.

### Critical items

- [x] **LOGIN-C1** — Replace SHA-256 with bcrypt (cost ≥ 12) for password storage; implement on-login migration for existing hashes
- [x] **LOGIN-C2** — Implement per-username failed-attempt counter and temporary lockout on the login endpoint

### High items

- [x] **LOGIN-H1** — Replace `==` password comparison with `crypto/subtle.ConstantTimeCompare`
- [x] **LOGIN-H2** — Upgraded in two stages: (1) MD5 → PBKDF2-SHA256 in `util/crypto.go`; (2) both `util/crypto.go` and `app-cli/settings/crypto.go` upgraded to Argon2id (32 MiB, 2 iterations) with per-encryption random salt and `ÿEG3` magic prefix; all existing ciphertext decrypts transparently via legacy paths
- [x] **LOGIN-H3** — Token key already stored in AES-256-GCM encrypted sidecar file by settings infrastructure; not in plaintext profile JSON
- [x] **LOGIN-H4** — Redirect following disabled on logon POST; 3xx responses return an error telling the user to update their server URL
- [x] **WEBAUTH-H1** — `passkeyGuard()` added; all five ceremony handlers return 404 when `ego.server.allow.passkeys` is false
- [x] **WEBAUTH-H2** — `credential.Authenticator.CloneWarning` checked after `FinishDiscoverableLogin`; login rejected with 401 on clone detection
- [x] **HTTP-H1** — `http.MaxBytesReader` wraps `r.Body` before `io.ReadAll`; returns 413 on oversize body; limit defaults to 32 MiB, configurable via `ego.server.max.body.size`
- [x] **HTTP-H2** — `makeHTTPServer()` helper constructs `http.Server` with `ReadHeaderTimeout` (10 s), `ReadTimeout` (30 s), `WriteTimeout` (120 s), and `IdleTimeout` (120 s); all three listeners (plain HTTP, TLS, and HTTP→HTTPS redirect) use it; all four values are configurable via `ego.server.{read.header|read|write|idle}.timeout`
- [x] **ASSET-H1** — `end` clamped to `totalSize - 1` in `readAssetRange` after `os.Stat`; `size` computed as `end - start + 1` (inclusive); eliminates the ~9 EB `make` allocation triggered by open-ended Range headers

### Medium items

- [x] **LOGIN-M1** — HTTP fallback removed from `resolveServerName`; unqualified names only try HTTPS. Explicit `http://` scheme still accepted as the user's deliberate choice.
- [x] **LOGIN-M2** — Removed `strings.TrimSpace` from password handling; prompt loop now uses `pass == ""` so spaces-only passwords are accepted as-is
- [x] **LOGIN-M3** — Cache now stores `*tokens.Token`; cache hits check `Expires` directly (no re-decryption). Blacklist is already handled: `tokens.Blacklist()` purges the token cache at revocation time.
- [x] **LOGIN-M4** — `{quoted}` format now logs an `auth.password.plaintext` warning on every use; bcrypt migration on first successful login was already in place from LOGIN-C1. Remove the special-case block once no `{quoted}` entries remain in the user database.
- [x] **WEBAUTH-M1** — `webAuthnBeginGuard()` added: per-IP sliding-window rate limit (10 req/min) and global pending-ceremony cap (200) enforced on both begin endpoints; returns 429 on breach
- [x] **WEBAUTH-M2** — Startup warning emitted via `server.webauthn.no.rpid` log key when passkeys are enabled but `ego.server.webauthn.rpid` is not configured
- [x] **WEBAUTH-M3** — `caches.SetExpiration` moved from `storeChallenge` to `defineNativeAdminHandlers` (server startup); called exactly once
- [x] **HTTP-M1** — `addSecurityHeaders()` in `serve.go` sets `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Content-Security-Policy`, and (TLS only) `Strict-Transport-Security` on every response
- [x] **HTTP-M2** — `X-Ego-Server` header removed entirely.
- [x] **HTTP-M3** — Resolved as a side-effect of HTTP-H2: `redirectToHTTPS` now builds its listener via `makeHTTPServer()`, which applies all four timeout values
- [x] **ASSET-M1** — `reportEnd` local variable introduced in `AssetsHandler`; clamped to `totalSize - 1` before formatting the `Content-Range` header, making responses RFC 7233-compliant for open-ended ranges
- [x] **ASSET-M2** — Error branch in `AssetsHandler` now writes `{"err": "asset not found"}` to the response; full OS error (including filesystem path) kept in `AssetLogger` only

### Low / Informational items

- [x] **LOGIN-L1** — Warning emitted via `ui.Say("logon.password.env")` when `EGO_PASSWORD` is set; env var cleared with `os.Unsetenv()` immediately after reading so child processes do not inherit it
- [x] **LOGIN-L2** — `ui.Say("rest.tls.insecure")` emitted (always visible) when insecure mode is activated via profile setting (`exchange.go`) or `EGO_INSECURE_CLIENT` env var (`client.go`); REST-log entry added for the Ego-program `verify: false` path (`methods.go`)
- [x] **LOGIN-L3** — `LogonHandler` now calls `tokens.Unwrap` on the freshly-minted token and uses `t.Expires` directly for `response.Expiration`; the independent duration re-calculation has been removed
- [x] **WEBAUTH-L1** — `challengeCookie` now accepts a `secure bool` parameter; `isSecureRequest()` helper sets it from `r.TLS != nil || X-Forwarded-Proto: https`; all four call sites updated
- [x] **WEBAUTH-L2** — Expiration refresh in `caches/find.go` moved outside the `ui.IsActive(ui.CacheLogger)` block so it always executes on a cache hit
- [x] **WEBAUTH-L3** — `WebAuthnClearPasskeysHandler` emits a `SERVER`-level `server.webauthn.admin.cleared.passkeys` log entry (visible in the dashboard Log tab) when an admin removes another user's passkeys
- [x] **HTTP-L1** — Generic `"not found"` / `"forbidden"` returned to client; raw URL path kept only in the `server.route.error` log entry
- [x] **HTTP-L2** — `mustAuthenticate` and `mustBeAdmin` failure branches in `ServeHTTP` now `return` immediately after sending the error response; request body is never read for rejected requests
- [x] **ASSET-L1** — Second `os.ReadFile(fn)` call in `readAssetFile` removed; function now returns the `data` and `err` captured by the first read
- [x] **ASSET-L2** — `normalizeAssetPath` rewritten to use `filepath.Clean(filepath.Join(root, path))` with a `strings.HasPrefix` confinement check; returns a guaranteed-nonexistent path on escape, treated uniformly as 404 by the caller

### Profile encryption items

- [x] **PROFILE-H1** — `encrypt`/`decrypt` in `app-cli/settings/crypto.go` now use `sha256.Sum256` for key derivation; new ciphertext carries a `"v2:"` prefix; `Decrypt` falls back to legacy MD5 path for prefix-less values
- [x] **PROFILE-M1** — Both read paths (`readOutboardConfigFiles`, `Load`) now call `NeedsNewHash()` after decrypt and immediately write back with the SHA-256 scheme; file path uses `os.WriteFile`, database path uses `UPDATE` after cursor close

### Tables endpoint items

- [x] **TABLES-C1** — Change `CommitHandler` guard from `len(parameters) != 0` to `!= 1`; matches `RollbackHandler` and prevents panic on `parameters[0]` with empty slice
- [x] **TABLES-H1** — Route schema substitution in `listTables` through `parsing.QueryParameters` (which calls `SQLEscape`) instead of bare `strings.ReplaceAll`
- [x] **TABLES-H2** — Add missing `!` to `Authorized` call in `getTableNames` so tables the user cannot read are filtered out, not those they can
- [x] **TABLES-M1** — Replace `"DROP TABLE " + tableName` in the DSN branch of `DeleteTable` with a quoted identifier; route through `parsing.QueryParameters` for consistency
- [x] **TABLES-M2** — Convert row ID filter in `formAbstractUpdateQuery` to a `$N` numbered parameter passed to `db.Exec` rather than string-embedded in the WHERE clause
- [x] **TABLES-M3** — Replace `defs.AdminAgent` with `defs.TableAdminPermission` in the `DeleteTable` authorization check; correct the error message from "read permission" to "admin permission"
- [x] **TABLES-M4** — Enforce a server-side maximum row limit in `PagingClauses`; default to 1000 rows when no limit is specified
- [x] **TABLES-L1** — Log full database errors server-side and return generic messages in HTTP error responses from the tables package
- [x] **TABLES-L2** — Wrap table and index names in double-quotes in all three SQLite `PRAGMA` format strings in `describe.go`
