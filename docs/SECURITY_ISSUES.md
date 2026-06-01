# Security Issues

This document records known security weaknesses in Ego. It is intended as a
living reference for future developers: each section describes the risk, the
affected code, and a concrete recommendation. A checklist at the bottom tracks
remediation progress.

## Structure of this document

There is a section (##) for each area of Ego that was evaluated for security and
operational risk issues. Within each section, the risks are grouped by severity
(critical, high, medium, and low). In each severity area, each exposure area is
documented with a unique identifier. For example, LOGIN-C1 is the first critical
issue in the LOGIN section. The information usually includes affected files, a
description detailed enough to help understand the issue, and recommended
remediation. When a fix is made, this may include adding additional details
about the resolution, particularly if the process of resolving it changes the
remediation plan somewhat.

Additionally, there is a section at the end of this document that shows the
completion of all issues. This section is grouped by severity, so all critical
items are grouped together, followed by all high items, etc.

## Index of Risk Areas

1. [Logins and Passwords (LOGIN)](#login)
2. [Webauthn and Passkeys (WEBAUTHN)](#webauthn)
3. [HTTP Server (HTTP)](#http)
4. [Tables Server (TABLES)](#tables)
5. [Assets Server (ASSET)](#asset)
6. [Profile Encryption (PROFILE)](#profile)
7. [Dashboard Code Execution (CODE)](#code)
8. [OAuth2 Authorization Server and Resource Server (OAUTH)](#oauth)
9. [Remediation Checklist](#checklist)

---

## Security Issues - Logins and Passwords<a name="login"></a>

### Critical (Login)

#### LOGIN-C1 — Weak password hashing (SHA-256, no salt)

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

#### LOGIN-C2 — No brute-force or rate-limiting protection on the login endpoint

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

### High (Login)

#### LOGIN-H1 — Timing attack in password comparison

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

#### LOGIN-H2 — Weak key derivation in AES encryption

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

#### LOGIN-H3 — Token signing key stored in plaintext configuration

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

#### LOGIN-H4 — Login credentials forwarded on HTTP 301 redirect to arbitrary host

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

### Medium (Login)

#### LOGIN-M1 — HTTP downgrade when HTTPS connection fails

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

#### LOGIN-M2 — Password whitespace stripped before hashing

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

#### LOGIN-M3 — Token cache bypasses expiry and revocation checks

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

#### LOGIN-M4 — Quoted-password legacy format allows plaintext storage

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

### Low / Informational (Login)

#### LOGIN-L1 — Password supplied via environment variable

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

#### LOGIN-L2 — `InsecureSkipVerify` available without prominent warning

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

#### LOGIN-L3 — Returned token expiration recalculated independently of token contents

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

## Security Issues — WebAuthn / Passkeys<a name="webauthn"></a>

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

## Security Issues — HTTP Server<a name="http"></a>

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

## Security Issues — Tables Endpoint <a name="tables"></a>

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

## Security Issues — Asset Handler<a name="asset"></a>

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

## Security Issues — Dashboard Code Execution<a name="code"></a>

This section records security weaknesses in the `POST /admin/run` endpoint that
compiles and executes Ego source code submitted from the dashboard Code and
Console tabs. The handler is implemented in `server/admin/run.go`, with sandbox
enforcement spread across `bytecode/context.go`, `runtime/exec/`, and
`runtime/io/`. Issues are rated using the same severity scale as the sections
above.

---

### High (Code)

#### CODE-H1 — Exec sandbox bypass when exec is globally permitted

**Affected files:**

- `runtime/exec/run.go:22` — `run()`
- `runtime/exec/output.go:18` — `output()`
- `runtime/exec/command.go:19` — `command()`

**Description:**  
Every execution context created by `RunCodeHandler` calls `.Sandboxed(true)`,
which sets `SandboxedExecSymbolName = true` in the context's symbol table.
The intent is to prevent user-submitted code from spawning OS subprocesses. In
practice the guard in all three exec functions reads:

```go
if !settings.GetBool(defs.ExecPermittedSetting) || !sandBoxedExec(s) {
    return nil, errors.ErrNoPrivilegeForOperation.In("Run")
}
```

`sandBoxedExec(s)` returns `true` when `SandboxedExecSymbolName` is `true`
(i.e. when the context is sandboxed), so `!sandBoxedExec(s)` evaluates to
`false`. When an administrator has also set `ExecPermittedSetting = true`, the
combined condition is `false || false = false` — the check passes and exec is
allowed even inside a sandboxed admin/run context.

The default value of `ExecPermittedSetting` is `false` (see
`runtime/profile/initialization.go:92`), so the endpoint is safe out of the
box. However, `.Sandboxed(true)` provides a false sense of protection: a
single administrator setting `ego.runtime.exec = true` silently re-enables
subprocess execution for all user-submitted dashboard code.

**Recommendation:**  
Make sandboxed execution contexts unconditionally block exec, regardless of the
global setting. One clear approach is to rename the symbol to reflect its actual
semantics (e.g., `SandboxedExecAllowed`) and then invert the guard so that a
sandboxed context explicitly overrides the global permission:

```go
// Block exec when the context is sandboxed, even if globally permitted.
if sandboxedCtx || !settings.GetBool(defs.ExecPermittedSetting) {
    return nil, errors.ErrNoPrivilegeForOperation.In("Run")
}
```

Alternatively, introduce a separate `sandboxedCtx` atomic bool on the
`bytecode.Context` (distinct from `sandboxedExec`) that is set by `.Sandboxed(true)`
and checked unconditionally by all exec functions before the global setting.

**Resolution (April 2026):**  
`Sandboxed()` in `bytecode/context.go` now sets `sandboxedExec` to `false`
(exec blocked) when `flag` is `true`, and restores it from `ExecPermittedSetting`
when `flag` is `false`. This ensures that calling `.Sandboxed(true)` on an
admin/run execution context unconditionally disables subprocess exec regardless
of the global setting. The existing exec guard in `runtime/exec/run.go`,
`output.go`, and `command.go` is unchanged; `!sandBoxedExec(s)` now correctly
evaluates to `true` for any sandboxed context.

---

### Medium (Code)

#### CODE-M1 — No request body size limit on `POST /admin/run`

**Affected file:** `server/admin/run.go:163` — `RunCodeHandler()`

```go
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
```

**Description:**  
The request body is decoded with no preceding call to `http.MaxBytesReader`.
Any user who holds the `ego.server.admin` or `ego.code` permission can POST an
arbitrarily large body. The full body is buffered before the JSON decoder
returns, so a multi-megabyte `Code` field will be compiled and executed (or
at least compiled). A sustained flood of large requests can exhaust server
memory without triggering the global body-size limit applied at the transport
layer by HTTP-H1, because `RunCodeHandler` re-reads `r.Body` directly rather
than consuming the pre-read `session.Body` buffer used by most other handlers.

**Recommendation:**  
Wrap the body before decoding, and add a post-decode length check on the `Code`
field:

```go
const maxRunBodyBytes = 1 << 18  // 256 KiB — generous for any plausible script
r.Body = http.MaxBytesReader(w, r.Body, maxRunBodyBytes)
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    return util.ErrorResponse(w, session.ID, err.Error(), http.StatusRequestEntityTooLarge)
}
if len(req.Code) > maxRunCodeBytes {
    return util.ErrorResponse(w, session.ID, "code too large", http.StatusRequestEntityTooLarge)
}
```

**Resolution (April 2026):**  
`RunCodeHandler` now wraps `r.Body` with `http.MaxBytesReader` (256 KiB limit)
before JSON decoding. A `*http.MaxBytesError` from the decoder returns 413;
other decode errors return 400. A post-decode `len(req.Code) > maxRunCodeBytes`
check also returns 413 for oversized code fields.

---

#### CODE-M2 — Global trace logger state mutated per-request without synchronization

**Affected file:** `server/admin/run.go:186` — `RunCodeHandler()`

```go
savedTrace := ui.IsActive(ui.TraceLogger)
ui.Active(ui.TraceLogger, req.Trace)
output, runErr := executeAdminEgo(session.ID, req.Code, req.Console, req.Trace, req.Session)
ui.Active(ui.TraceLogger, savedTrace)
```

**Description:**  
`ui.IsActive` and `ui.Active` operate on a single global logger-state map
shared across all goroutines. The read-modify-execute-restore sequence above is
not protected by any mutex. When two concurrent requests arrive with different
`Trace` values, one request can overwrite the other's saved state. This creates
two observable problems:

1. **Unintended trace exposure** — a request that did not ask for tracing may
   run with the trace logger enabled because a concurrent request enabled it
   after the first request saved `savedTrace = false`.
2. **Data race** — Go's race detector will flag concurrent reads and writes to
   the shared logger state as a data race (no synchronization).

Since the execution context already accepts a per-request trace flag via
`ctx.SetTrace(trace)`, the global mutation is unnecessary for controlling
per-execution tracing. The `ui.Active` calls are only needed if some code path
outside the context checks the global flag directly.

**Recommendation:**  
Remove the global `ui.Active` mutations from `RunCodeHandler`. Pass the trace
flag exclusively through the `bytecode.Context` (`ctx.SetTrace(req.Trace)`)
so each request controls its own tracing without touching shared state. If
global trace output is still required for some paths, protect the
save/restore pair with a dedicated mutex.

**Resolution (April 2026):**  
The save/restore of `ui.Active(ui.TraceLogger)` has been replaced with a
package-level `traceRunMu sync.Mutex`. When `req.Trace` is true, the handler
acquires the mutex, sets the logger active, and defers both the restore and the
unlock. Non-trace requests proceed without serialization. This eliminates the
data race while preserving global trace output for the bytecode run-loop log
messages that check `ui.IsActive(ui.TraceLogger)` directly.

---

#### CODE-M3 — Client-supplied session UUID not validated or bound to the authenticated user

**Affected file:** `server/admin/run.go:179` — `RunCodeHandler()`

```go
if req.Session == "" {
    req.Session = uuid.New().String()
}
```

**Description:**  
The `Session` field is taken verbatim from the JSON request body and used
directly as the key into both `codeSessions` (persistent symbol tables) and
`debugSessions` (active debugger contexts). No format validation is performed —
the field accepts any string. More importantly, there is no binding between a
session key and the authenticated user who created it.

Two consequences follow:

1. **Session fixation** — a malicious user can specify a session UUID they
   already know (e.g. one observed or guessed from another user's traffic) and
   interact with that user's persistent symbol table or inject commands into
   their active debug session.
2. **Log injection** — the raw UUID is written to the SERVER log via
   `ui.A{"id": uuid}`. A crafted value containing newline characters or
   log-format control sequences can corrupt structured log output.

**Recommendation:**  
Validate that the client-supplied `Session` value conforms to UUID v4 format
before accepting it (reject with 400 otherwise). Additionally, bind each session
entry to the authenticated username at creation time and enforce that the
requesting user matches the session owner on every subsequent call:

```go
if entry.owner != session.User {
    return util.ErrorResponse(w, session.ID, "session not found", http.StatusNotFound)
}
```

Returning 404 rather than 403 avoids confirming the existence of another user's
session.

**Resolution (April 2026):**  
`RunCodeHandler` now calls `uuid.Parse(req.Session)` before using the value
and returns 400 for any non-UUID string. `codeSessionEntry` and `debugSession`
both gained an `owner string` field set to `session.User` at creation time.
`getOrCreateSymbolTable` and `executeAdminDebug` each check `entry.owner != user`
on session lookup and return an opaque error (`run.not.found` /
`ErrNoPrivilegeForOperation`) that does not reveal whether the session belongs
to another user. The localized `run.not.found` key has been added to all three
language files.

---

#### CODE-M4 — Sandbox I/O path confinement can be bypassed via symlinks

**Affected file:** `runtime/io/io.go:121` — `sandboxName()`

```go
func sandboxName(flag bool, path string) string {
    if sandboxPrefix := settings.Get(defs.SandboxPathSetting); flag && sandboxPrefix != "" {
        if strings.HasPrefix(path, "../") || ... {
            path = strings.ReplaceAll(path, "..", "<invalid path>")
        }
        if strings.HasPrefix(path, sandboxPrefix) {
            return path
        }
        return filepath.Join(sandboxPrefix, path)
    }
    return path
}
```

**Description:**  
`sandboxName` prevents `..`-based directory traversal in the path string itself,
but does not resolve symlinks before checking or returning the path. If user
code (or a prior operation) creates a symlink inside the sandbox directory that
points to a location outside it, subsequent `io.Open` and `io.ReadDir` calls
will follow that symlink and access the target path, bypassing the confinement
entirely. For example:

```text
sandbox/escape -> /etc
io.Open("escape/passwd")  // sandboxName returns sandbox/escape/passwd
                           // os.OpenFile follows symlink → /etc/passwd
```

This is a well-known weakness of path-prefix confinement without symlink
resolution: the check is done on the path string rather than on the filesystem
object it ultimately refers to.

**Recommendation:**  
After computing the candidate path, resolve all symlinks with
`filepath.EvalSymlinks` and verify the result still falls under the sandbox
root before opening or listing:

```go
resolved, err := filepath.EvalSymlinks(candidate)
if err != nil || !strings.HasPrefix(resolved, sandboxPrefix+string(filepath.Separator)) {
    return filepath.Join(sandboxPrefix, "__invalid__")
}
return resolved
```

Note that `filepath.EvalSymlinks` requires the file to already exist; for
write operations (creating new files) that will not yet exist, verify the
parent directory instead.

---

#### CODE-M5 — Language extensions enabled in sandboxed symbol table

**Affected file:** `server/admin/run.go:355` — `getOrCreateSymbolTable()`

```go
comp := compiler.New("dashboard").
    SetExtensionsEnabled(true).
    SetRoot(consoleTable)
```

**Description:**  
Persistent console symbol tables are initialized with the compiler's extension
mode enabled. Extensions add language features beyond the standard Ego/Go
subset (for example, `panic` as a statement token is guarded by
`ExtensionsEnabledSetting` in `compiler/statement.go:72`). Because the symbol
table is re-used across multiple requests in console mode, any effect of
extension-enabled compilation persists into subsequent executions.

If any extension exposes lower-level primitives, bypasses type or sandbox
checks, or widens the set of callable native functions in ways not anticipated
by the sandbox model, every dashboard user who has a console session is
exposed to that wider attack surface. The risk is currently unquantified
because the full set of behaviors gated on `ExtensionsEnabledSetting` has not
been audited for sandbox compatibility.

**Recommendation:**  
Audit every code path that checks `ExtensionsEnabledSetting` or
`SetExtensionsEnabled` and confirm that none of the extension-only behaviors
conflict with the `Sandboxed(true)` constraints. If any do, disable extensions
for sandboxed contexts, or guard the individual extension features with an
additional sandbox check.

---

### Low / Informational (Code)

#### CODE-L1 — Full user-submitted code body written to REST log

**Affected file:** `server/admin/run.go:167` — `RunCodeHandler()`

```go
if ui.IsActive(ui.RestLogger) {
    b, _ := json.MarshalIndent(req, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
    ui.Log(ui.RestLogger, "rest.request.payload", ui.A{
        "session": session.ID,
        "body":    string(b),
    })
}
```

**Description:**  
When the REST logger is active, the entire deserialized request — including the
`Code` field — is written to the server log. If a user submits code that
contains sensitive values (database credentials, API keys, personal data
embedded in test scripts), those values are persisted in the server log files
for the duration of the log retention period. This is particularly notable
because log files are typically accessible to a broader audience than the
dashboard session itself.

**Recommendation:**  
Redact or truncate the `Code` field before logging. A reasonable approach is to
log the first 120 characters and append an ellipsis when the field is longer:

```go
logReq := req
if len(logReq.Code) > 120 {
    logReq.Code = logReq.Code[:120] + "…"
}
b, _ := json.MarshalIndent(logReq, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
```

This preserves enough context to identify the request in the log without
capturing the full script content.

**Resolution (April 2026):**  
`RunCodeHandler` now copies the request into a local `logReq` variable and
truncates `logReq.Code` to 120 characters (appending `"..."`) before passing
it to `json.MarshalIndent`. The original `req.Code` is unmodified and used
for execution as before.

---

## Security Issues — OAuth2 Authorization Server and Resource Server<a name="oauth"></a>

This section records security weaknesses in the OAuth2 Authorization Server (AS)
implemented in `server/oauth/authserver/` and the OAuth2 Resource Server (RS) client
implemented in `server/oauth/` and `server/oauth/rshandlers/`. The AS provides
standard OAuth2/OIDC endpoints; the RS validates JWT Bearer tokens issued by an
external identity provider and drives the Authorization Code + PKCE login flow.
Issues are rated using the same severity scale as sections above.

---

### High (OAuth)

#### OAUTH-H1 — No rate limiting on the AS login form endpoint

**Affected file:** `server/oauth/authserver/authorize.go:181` — `AuthorizePostHandler()`

**Description:**
`AuthorizePostHandler` calls `auth.ValidatePassword` directly without invoking
the rate-limiting infrastructure (`CheckRateLimit` / `RecordFailure` / `RecordSuccess`)
that `router/auth.go:Authenticate` uses for the native auth path. An attacker who
can reach the AS can submit an unlimited number of credential guesses through
`POST /oauth2/authorize` at full network speed — the per-account lockout added by
LOGIN-C2 simply does not apply here. Because the handler re-renders the form on
failure rather than returning 401, automated tools can drive the form POST loop
without any HTTP-level friction and without triggering the native auth lockout
counters.

The bcrypt validation that backs `auth.ValidatePassword` is inherently slow
(~100 ms per attempt), which provides a modest natural throttle, but that
alone is not adequate protection against distributed attacks.

**Recommendation:**
Call `CheckRateLimit(username)` before attempting `validatePassword` and return
a 429 response when the account is locked. Call `RecordSuccess(username)` and
`RecordFailure(sessionID, username)` after the validation result is known. This
is exactly the pattern in `router/auth.go:272–278` and can be extracted into a
shared helper so both paths stay in sync.

**Resolution (May 2026):**
`AuthorizePostHandler` now calls `router.CheckRateLimit(username)` before
attempting credential validation. A locked account causes the login form to
re-render with a lockout message (via the new `reRenderWithError` helper) rather
than proceeding to `validatePassword`. After a failed credential check,
`router.RecordFailure(session.ID, username)` is called so the counter advances.
After a successful credential check, `router.RecordSuccess(username)` clears the
counter. The per-account lockout budget is now shared between the native auth
path and the OAuth2 form login path.  New log message key
`oauth.as.authorize.locked` added to all three language files. Tests in
`server/oauth/authserver/authorize_test.go`.

---

#### OAUTH-H2 — Revoked JWT tokens bypass the RS validation cache

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

---

#### OAUTH-H3 — PKCE not required for public clients in the AS authorization flow

**Affected file:** `server/oauth/authserver/codes.go:168` — `verifyPKCE()`

```go
func verifyPKCE(pending PendingAuthorization, codeVerifier string) error {
    if pending.CodeChallenge == "" {
        // No PKCE was used in this authorization request — nothing to verify.
        return nil
    }
```

**Description:**
PKCE (RFC 7636) is enforced only when the client chose to include a
`code_challenge` in the authorization request. Public clients — those registered
without a `client_secret_hash`, such as the built-in `ego-cli` — can omit PKCE
entirely. Without PKCE, an attacker who intercepts or steals the authorization
code (e.g., via a malicious redirect-URI registration at the OS level, or log
exposure) can exchange it for tokens at the token endpoint without possessing the
original code_verifier.

RFC 9700 (OAuth 2.0 Security Best Current Practice) §2.1.1 mandates that all
public clients use PKCE. PKCE is also strongly recommended for confidential
clients. Accepting a code from a public client without PKCE removes this
protection entirely.

**Recommendation:**
If the client is public (`ClientSecretHash == ""`), `handleAuthorizationCodeGrant`
must reject the exchange when `pending.CodeChallenge == ""`:

```go
if client.ClientSecretHash == "" && pending.CodeChallenge == "" {
    return util.ErrorResponse(w, session.ID,
        i18n.T("oauth.as.pkce.required"), http.StatusBadRequest)
}
```

For confidential clients, PKCE should be strongly recommended via documentation;
making it mandatory for all clients is also an option and is the safest default.

**Resolution (May 2026):**
`handleAuthorizationCodeGrant` in `server/oauth/authserver/token.go` now checks
`client.ClientSecretHash == "" && pending.CodeChallenge == ""` after validating
the redirect URI. When both conditions hold (public client, no PKCE in the
authorization request), the exchange is rejected with 400 and a log entry under
`oauth.as.pkce.missing`. The `verifyPKCE` function is unchanged — it still
validates PKCE when a challenge is present. Confidential clients are unaffected.
New error key `oauth.as.pkce.required` and log key `oauth.as.pkce.missing` added
to all three language files. Tests in `server/oauth/authserver/token_test.go`.

---

#### OAUTH-H4 — CSRF token not regenerated when login form is re-rendered on failure

**Affected file:** `server/oauth/authserver/authorize.go:233` — `AuthorizePostHandler()`

```go
data := loginFormData{
    ClientID:            clientID,
    RedirectURI:         redirectURI,
    Scope:               scope,
    State:               state,
    CodeChallenge:       codeChallenge,
    CodeChallengeMethod: codeChallengeMethod,
    Error:               "Invalid username or password.",
    // CSRFToken is zero-value ("") — the field is not set
}
```

**Description:**
When credential validation fails, `AuthorizePostHandler` re-renders the login form
with an error message but does not generate a new CSRF token. The `CSRFToken`
field of `loginFormData` is left at its zero value, producing an empty
`<input type="hidden" name="csrf_token" value="">` in the re-rendered HTML.

The CSRF cookie set during the original GET still holds the original random
token. When the user corrects their credentials and submits the re-rendered form,
the POST handler compares the cookie value (non-empty) against the form value
(`""`) — they do not match — and returns 403 Forbidden. The user cannot retry
without navigating back and restarting the entire authorization flow from scratch,
which is not indicated anywhere in the error UI.

Two consequences follow:

1. **Correctness / usability:** Any password mistake permanently breaks the in-
   progress login session. The user must restart the full browser-based flow.
2. **Security:** Because the CSRF re-render path does not set a new CSRF cookie,
   an automated attacker who drives the GET → POST loop would also need to restart
   the GET after every failed attempt — providing a minor additional friction but
   not a meaningful security control.

**Recommendation:**
In the failure branch, generate a fresh CSRF token, set a new cookie, and include
the token in `loginFormData`:

```go
newCSRF, _ := generateCSRFToken()
http.SetCookie(w, &http.Cookie{
    Name: csrfCookieName, Value: newCSRF,
    Path: "/oauth2/authorize", HttpOnly: true, SameSite: http.SameSiteStrictMode,
})
data := loginFormData{ ..., CSRFToken: newCSRF, Error: "Invalid username or password." }
```

This ensures that each form render (whether first-load or after failure) pairs a
fresh nonce in the cookie with the same nonce embedded in the form.

**Resolution (May 2026):**
All re-render paths in `AuthorizePostHandler` are now routed through the new
`reRenderWithError` helper in `server/oauth/authserver/authorize.go`. The helper
generates a fresh CSRF token via `generateCSRFToken`, replaces the CSRF cookie in
the response, and populates `loginFormData.CSRFToken` with the new nonce. This
is called for both the rate-limit lockout path (OAUTH-H1) and the bad-credential
path, ensuring the form is always submittable after an error. Tests in
`server/oauth/authserver/authorize_test.go`.

---

### Medium (OAuth)

#### OAUTH-M1 — Audience validation skipped by default

**Affected file:** `server/oauth/jwt.go:86` — `parseAndValidateJWT()`

```go
if audience != "" {
    parserOpts = append(parserOpts, jwt.WithAudience(audience))
}
```

**Description:**
When `ego.server.oauth.audience` is not set (the default), `cfg.Audience` is an
empty string and audience validation is entirely skipped. A JWT issued for any
other resource server by the same IdP — including a test or staging environment —
will be accepted as valid by the production Ego RS. Per RFC 9700 §2.8, "Resource
servers MUST validate the audience claim" because it is the principal mechanism
that prevents token confusion attacks across services sharing the same IdP.

**Recommendation:**
Document `ego.server.oauth.audience` as a **required** setting in any production
deployment. Add a startup warning (analogous to `WEBAUTH-M2`) when
`ego.server.oauth.provider` is set but `ego.server.oauth.audience` is empty:

```go
if cfg.Provider != "" && cfg.Audience == "" {
    ui.Log(ui.ServerLogger, "oauth.rs.no.audience", ui.A{})
}
```

Optionally, refuse to start in `resource-server` or `hybrid` mode unless the
audience is configured, treating it the same way as a missing issuer.

**Resolution (May 2026):**
`commands/server.go` now calls `oauth.GetConfig().Audience == ""` immediately
after a successful `oauth.Initialize()`.  When the audience is unconfigured,
`ui.Log(ui.ServerLogger, "oauth.rs.no.audience", ...)` emits a SERVER-level log
entry that is always visible in the server log and the dashboard Log tab.  The
new message key `oauth.rs.no.audience` (with a `{{provider}}` argument) was
added to all three language files.  The server starts normally — a hard refusal
would be a breaking change for existing deployments that have not yet set the
audience — but the warning makes the gap impossible to miss.

---

#### OAUTH-M2 — No timeout on outbound IdP HTTP calls

**Affected files:**

- `server/oauth/discovery.go:79` — `discoverEndpoints()`: `http.Get(discoveryURL)`
- `server/oauth/jwks.go:89` — `refreshJWKS()`: `http.Get(jwksURL)`
- `server/oauth/flow_authcode.go:143` — `ExchangeCode()`: `http.DefaultClient.Do(req)`
- `app-cli/app/logon_oauth.go:201` — `fetchOIDCDiscovery()`: `http.Get(discoveryURL)`
- `app-cli/app/logon_oauth.go:374` — `postTokenRequest()`: `http.DefaultClient.Do(req)`

**Description:**
All outbound HTTP calls to the identity provider (discovery, JWKS fetch, and token
exchange) use either `http.Get` or `http.DefaultClient.Do`, neither of which sets a
deadline. If the IdP is slow or unresponsive, each goroutine handling an inbound
request blocks indefinitely on the outbound call. A network partition, an
overloaded IdP, or a deliberate slowdown by a malicious upstream server can
exhaust all available server goroutines — effectively causing a DoS of the Ego
server without any involvement of the attacker's client.

This is analogous to the Slowloris vulnerability described in HTTP-H2, but for
outbound connections rather than inbound ones.

**Recommendation:**
Replace `http.DefaultClient` with a client that carries a context deadline derived
from the inbound request, or use a package-level client with a bounded timeout:

```go
var idpClient = &http.Client{Timeout: 10 * time.Second}
```

Apply this to `discoverEndpoints`, `refreshJWKS`, `ExchangeCode`, and the CLI's
`postTokenRequest`. The discovery and JWKS fetches happen at startup and on cache
miss; 10–30 seconds is a reasonable wall-clock limit. The token exchange happens
per-request; 10 seconds matches common IdP SLAs.

**Resolution (May 2026):**
A new file `server/oauth/client.go` defines a package-level `idpClient =
&http.Client{Timeout: 10 * time.Second}`.  All three server-side outbound call
sites — `discoverEndpoints` (`discovery.go`), `refreshJWKS` (`jwks.go`), and
`ExchangeCode` (`flow_authcode.go`) — now use `idpClient.Get(...)` /
`idpClient.Do(...)` instead of `http.Get` / `http.DefaultClient.Do`.  The CLI
gains a parallel `oauthHTTPClient = &http.Client{Timeout: 30 * time.Second}` in
`app-cli/app/logon_oauth.go` (30 s is more generous for a user-facing flow).
Both `fetchOIDCDiscovery` and `postTokenRequest` in the CLI now use
`oauthHTTPClient`.  Tests in `server/oauth/medium_test.go` verify that
`idpClient.Timeout` is non-zero and that the client is actually used for
discovery requests.

---

#### OAUTH-M3 — Revocation endpoint ignores HTTP Basic Auth for client authentication

**Affected file:** `server/oauth/authserver/revoke.go:31` — `RevokeHandler()`

```go
clientID := r.FormValue("client_id")
clientSecret := r.FormValue("client_secret")
```

**Description:**
The AS token endpoint (`handleAuthorizationCodeGrant`, `handleClientCredentialsGrant`,
`handleRefreshTokenGrant`) uses `validateBasicAuth(r)`, which prefers the HTTP
`Authorization: Basic` header and falls back to form-encoded `client_id` /
`client_secret` fields. This matches RFC 6749 §2.3.1.

The revocation endpoint (`POST /oauth2/revoke`) reads credentials only from form
values, completely ignoring any `Authorization: Basic` header. RFC 7009 §2.1
requires the revocation endpoint to use the same client authentication mechanism
as the token endpoint. Confidential clients that prefer Basic Auth cannot
authenticate at the revocation endpoint and will receive a 401 error even when
supplying valid credentials.

**Recommendation:**
Replace the two `r.FormValue` calls with a call to the existing `validateBasicAuth`
helper:

```go
clientID, clientSecret := validateBasicAuth(r)
```

This is a one-line fix that brings the revocation endpoint into alignment with
the token endpoint and RFC 7009.

**Resolution (May 2026):**
The two `r.FormValue("client_id")` / `r.FormValue("client_secret")` lines in
`RevokeHandler` (`server/oauth/authserver/revoke.go`) were replaced with a
single call to the existing `validateBasicAuth(r)` helper (defined in
`token.go`).  `validateBasicAuth` tries the `Authorization: Basic` header first
and falls back to form fields, so all existing clients that POST credentials
in the body continue to work without changes.  Tests in
`server/oauth/authserver/revoke_test.go` cover: Basic Auth accepted, form
credentials still accepted, wrong secret rejected, unknown client rejected, and
public-client Basic Auth with empty password accepted.

---

#### OAUTH-M4 — OIDC discovery and JWKS responses read without a size limit

**Affected files:**

- `server/oauth/discovery.go:86` — `discoverEndpoints()`: `io.ReadAll(resp.Body)`
- `server/oauth/jwks.go:99` — `refreshJWKS()`: `io.ReadAll(resp.Body)`

**Description:**
Both functions read the IdP's HTTP response body into memory with a bare
`io.ReadAll`, applying no size limit before the allocation. A malicious or
compromised IdP, or a network attacker who can intercept the outbound TLS
connection (e.g., via a CA compromise), can return an arbitrarily large response
body. The full response is allocated into a single byte slice, so even a
moderately large payload (tens of megabytes) causes a visible memory spike;
gigabyte payloads can exhaust heap and crash the server.

Discovery documents and JWKS responses are inherently small — a few kilobytes at
most for any realistic deployment. A generous limit of 1 MiB is far more than
any legitimate document requires.

**Recommendation:**
Wrap the response body with `http.MaxBytesReader` (or `io.LimitReader`) before
reading:

```go
body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB cap
```

A response that exceeds the limit should be treated as an error and logged so the
operator can investigate.

**Resolution (May 2026):**
Both `discoverEndpoints` (`discovery.go`) and `refreshJWKS` (`jwks.go`) now wrap
`resp.Body` in `&io.LimitedReader{R: resp.Body, N: maxDiscoveryBytes + 1}` (where
`maxDiscoveryBytes = 1 << 20`, 1 MiB) before calling `io.ReadAll`.  Using N+1
rather than N avoids an off-by-one: when `lr.N` reaches zero after reading,
it proves the body is strictly larger than the limit rather than exactly equal to
it.  A zero `lr.N` after `io.ReadAll` returns causes the function to return a
descriptive error containing "exceeds" so the operator knows why the document was
rejected.  Tests in `server/oauth/medium_test.go` cover: a body one byte over the
limit is rejected, a body exactly at the limit passes the size check (then fails
JSON parsing, confirming no off-by-one regression).

---

#### OAUTH-M5 — RS PKCE state store has no maximum size

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

---

### Low / Informational (OAuth)

#### OAUTH-L1 — CSRF cookie missing `Secure` flag on AS authorize handler

**Affected file:** `server/oauth/authserver/authorize.go:147` — `AuthorizeGetHandler()`

```go
http.SetCookie(w, &http.Cookie{
    Name:     csrfCookieName,
    Value:    csrfToken,
    Path:     "/oauth2/authorize",
    HttpOnly: true,
    SameSite: http.SameSiteStrictMode,
    // Secure is not set
})
```

**Description:**
The CSRF cookie used to protect the AS login form is `HttpOnly` and
`SameSite: Strict`, but the `Secure` attribute is not set. In any configuration
where the Ego AS accepts plain HTTP connections (before HTTPS redirect, or in
development), the CSRF nonce can be transmitted in cleartext. A network observer
can capture the cookie and the form nonce, enabling CSRF attacks against users on
unencrypted connections.

This is the same issue as WEBAUTH-L1, which was already resolved for the WebAuthn
challenge cookie. The `isSecureRequest(r)` helper added there can be reused here.

**Recommendation:**
Set the `Secure` attribute conditionally using `isSecureRequest(r)`:

```go
http.SetCookie(w, &http.Cookie{
    Name:     csrfCookieName,
    Value:    csrfToken,
    Path:     "/oauth2/authorize",
    HttpOnly: true,
    SameSite: http.SameSiteStrictMode,
    Secure:   isSecureRequest(r),
})
```

Apply the same fix to the re-render path in `AuthorizePostHandler` once
OAUTH-H4 is resolved and a new CSRF token is generated there as well.

**Resolution (May 2026):**
`isSecureRequest` in `router/webauthn.go` was renamed to `IsSecureRequest`
(exported) so that the `authserver` package can call it without duplicating the
logic.  All four internal call sites in `router/webauthn.go` and all three
references in `router/webauthn_test.go` were updated to use the new name.
Both cookie-setting locations in `authorize.go` — `AuthorizeGetHandler` and
`reRenderWithError` — now pass `Secure: router.IsSecureRequest(r)`.  The flag
is `true` when `r.TLS != nil` (direct TLS) or when the `X-Forwarded-Proto:
https` header is set (proxy-terminated TLS); it is `false` for plain HTTP so
development servers are not broken.  Tests in
`server/oauth/authserver/low_test.go` and
`server/oauth/authserver/secure_request_test.go` cover: HTTPS sets Secure,
plain HTTP omits Secure, X-Forwarded-Proto sets Secure, and
`router.IsSecureRequest` is callable from outside the router package.

---

#### OAUTH-L2 — AS token endpoint router registration uses JSON content type

**Affected file:** `server/oauth/authserver/authserver.go:109` — `RegisterRoutes()`

```go
r.New(defs.OAuthTokenPath, TokenHandler, http.MethodPost).
    Class(router.ServiceRequestCounter).
    AcceptMedia(defs.JSONMediaType)
```

**Description:**
The token endpoint (`POST /oauth2/token`) is registered in the router with
`AcceptMedia(defs.JSONMediaType)` (i.e., `application/json`). However, RFC 6749
§4.1.3 and §4.4.2 require clients to send token requests as
`application/x-www-form-urlencoded`. The handler already uses `r.ParseForm()` to
read the request body, which is consistent with form encoding, not JSON.

A strictly compliant OAuth2 client library that sets
`Content-Type: application/x-www-form-urlencoded` (as required) may be rejected
by the Ego router before `TokenHandler` is ever called, if the router enforces
the declared `AcceptMedia` type. The `Authorization Code` flow from the CLI is
unaffected in practice because the CLI's `postTokenRequest` sets
`Content-Type: application/x-www-form-urlencoded` and the router may be lenient
in practice, but third-party clients that rely on strict content-type enforcement
(e.g., `application/json`) on the wrong side of the mismatch will fail.

**Recommendation:**
Remove the `AcceptMedia(defs.JSONMediaType)` chain call from the token endpoint
registration, or replace it with the correct media type:

```go
r.New(defs.OAuthTokenPath, TokenHandler, http.MethodPost).
    Class(router.ServiceRequestCounter)
    // No AcceptMedia constraint — RFC 6749 requires form-encoded bodies
```

**Resolution (May 2026):**
The `.AcceptMedia(defs.JSONMediaType)` chain call was removed from the
`POST /oauth2/token` route registration in `RegisterRoutes`
(`server/oauth/authserver/authserver.go`).  The router now imposes no
restriction on the Accept header for this endpoint, matching the RFC 6749
requirement that clients send `application/x-www-form-urlencoded` request
bodies.  The handler (`TokenHandler`) uses `r.ParseForm()` internally and
always writes JSON responses, regardless of what the client declares in its
Accept header.  Tests in `server/oauth/authserver/low_test.go` verify that
the handler processes form-encoded requests without an Accept header and that
the error path (unsupported grant type) produces a grant-type error rather
than a media-type rejection.

---

---

### High (OAuth RS — second audit, June 2026)

#### OAUTH-H5 — Unvalidated `redirect` query parameter enables open redirect in RS authorize handler

**Affected file:** `server/oauth/rshandlers/authorize_handler.go:29-31`

```go
if override := r.URL.Query().Get("redirect"); override != "" {
    cfg.RedirectURI = override
}
```

**Description:**
`AuthorizeRedirectHandler` accepts an optional `redirect` query parameter and
substitutes it for the configured `ego.server.oauth.redirect.uri` without any
server-side validation. The substituted URI is then sent to the IdP as the
`redirect_uri` parameter of the authorization request.

Two separate problems arise:

1. **Open redirect / phishing.** If the IdP's client registration uses a
   wildcard, prefix match, or pattern for allowed redirect URIs (common with
   some providers, or when an operator accidentally registers too broadly), an
   attacker can supply an arbitrary URI. After the user authenticates with the
   IdP, the browser is redirected to the attacker-controlled URI. Because PKCE
   protects the subsequent code exchange, the attacker cannot obtain tokens, but
   they receive the authorization code in the URL and can display a convincing
   fake "login successful" page. Even with an exact-match IdP registration, the
   capability to redirect to any registered URI (including a staging endpoint)
   is unintended and violates the principle that a server should enforce its own
   policy, not rely solely on the IdP.

2. **Broken feature — stored redirect URI is silently ignored.** The overridden
   `RedirectURI` is stored in the PKCE `pendingState` as `ps.RedirectURI`
   (`state.go:119`). However, `CallbackHandler` retrieves the global config with
   `oauth.GetConfig()` and calls `ExchangeCodePublic(cfg, code, ps.CodeVerifier)`,
   which sends `cfg.RedirectURI` (the original configured value) to the IdP token
   endpoint. The stored `ps.RedirectURI` is never read. As a result, the token
   exchange always fails with a redirect-URI mismatch error from the IdP, making
   the override feature entirely inoperative.

**Recommendation:**
Remove the `redirect` override entirely from `AuthorizeRedirectHandler`. If
per-request redirect URI overrides are a future requirement, validate the
supplied URI against a server-side allowlist of permitted redirect URIs before
accepting it, AND pass `ps.RedirectURI` to `ExchangeCode` instead of
`cfg.RedirectURI` so that the same URI is used consistently across both legs of
the flow.

**Resolution (June 2026):**
Five coordinated changes were made:

1. **`server/oauth/rshandlers/authorize_handler.go`** — The `redirect` query
   parameter override is removed entirely.  `AuthorizeRedirectHandler` now
   always uses `cfg.RedirectURI` (the server-configured value) and never reads
   the `redirect` query parameter.  The function-level doc comment was updated
   to explain why no per-request override is accepted.  The internal error
   message for `BuildAuthorizeURL` failure was also changed to a generic string
   (no longer leaks the raw error to the browser — partial fix for OAUTH-L4).

2. **`server/oauth/rshandlers/routes.go`** — The `.Parameter("redirect",
   "string")` chain call was removed from the `GET /services/admin/oauth/authorize`
   route registration, so the router no longer declares that parameter as
   expected input.

3. **`server/oauth/state.go`** — `RedirectURI string` was removed from
   `pendingState` (the internal struct) and `newState()` no longer accepts a
   `redirectURI` argument.  The field was dead storage: it was set by
   `AuthorizeURL` but never read by `CallbackHandler`.

4. **`server/oauth/oauth.go`** — `RedirectURI string` was removed from the
   exported `PendingState` struct.  `ValidateCallbackState` no longer copies
   the field.

5. **`server/oauth/flow_authcode.go`** — The `newState(cfg.RedirectURI)` call
   was updated to `newState()`.

Tests in `server/oauth/rshandlers/authorize_handler_test.go` cover:
`TestAuthorizeRedirectIgnoresRedirectParam` (primary regression — verifies that
`?redirect=<attacker-uri>` has no effect on the redirect_uri embedded in the
authorization URL), `TestAuthorizeRedirectUsesConfiguredURI` (happy path with
all required PKCE parameters present), `TestAuthorizeRedirectNoProvider` (503
when provider is not configured), `TestAuthorizeRedirectNoRedirectURI` (500
when redirect URI is not configured), and
`TestAuthorizeRedirectTwoCallsProduceDifferentStates` (each flow gets a unique
PKCE state token).

Existing tests in `server/oauth/state_test.go` and `medium_test.go` were
updated to match the new `newState()` signature.

---

### Medium (OAuth RS — second audit, June 2026)

#### OAUTH-M6 — Token exchange response body read without size limit

**Affected file:** `server/oauth/flow_authcode.go:155` — `ExchangeCode()`

```go
body, err := io.ReadAll(resp.Body)
```

**Description:**
`ExchangeCode` reads the IdP token endpoint response with a bare `io.ReadAll`,
applying no byte limit before the allocation. OAUTH-M4 fixed the same class of
issue for the OIDC discovery document and JWKS responses, but the token exchange
response was not updated at the same time.

A malicious or compromised IdP token endpoint (or a network attacker who can
intercept the server-to-server TLS connection) can return an arbitrarily large
response body. The `idpClient` timeout (10 s, added by OAUTH-M2) bounds the
total wall-clock time, but a slowly-streaming attacker can deliver megabytes
within that window. The full body is allocated as a single byte slice, so a
large payload causes a memory spike that can be repeated on every user login that
triggers a token exchange (i.e., every Authorization Code flow completion).

**Recommendation:**
Apply the same `io.LimitedReader` pattern used in `discoverEndpoints` and
`refreshJWKS`:

```go
const maxTokenBodyBytes = 64 << 10  // 64 KiB — generous for any real token response
lr := &io.LimitedReader{R: resp.Body, N: maxTokenBodyBytes + 1}
body, err := io.ReadAll(lr)
if err != nil {
    return "", "", errors.New(errors.ErrOAuthTokenRead).Context(err.Error())
}
if lr.N == 0 {
    return "", "", errors.New(errors.ErrOAuthTokenSizeLimit).Context(doc.TokenEndpoint)
}
```

A real token endpoint response is a small JSON object (access token, expiry,
scopes) — 64 KiB is many times larger than any legitimate response.

---

#### OAUTH-M7 — Every JWT with an unknown `kid` triggers an unconditional JWKS refresh

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

---

#### OAUTH-M8 — Custom permission claim names silently unsupported; all JWT holders granted minimum "ego.logon"

**Affected file:** `server/oauth/claims.go:96-114` — `extractPermissionTokens()`

```go
default:
    // Custom claim names return an empty slice.
    return []string{}
```

**Description:**
`extractPermissionTokens` handles only two claim names: `"scope"` (standard
OAuth2, space-delimited) and `"roles"` (common IdP extension, string array). Any
other value for `ego.server.oauth.permission.claim` — including `"groups"`,
`"authorities"`, `"realm_access.roles"` (Keycloak), or provider-specific names —
silently returns an empty slice.

The empty slice propagates to `mapClaimsToPermissions`, which finds no matching
tokens and applies an unconditional fallback:

```go
if len(permissions) == 0 {
    permissions = []string{"ego.logon"}
}
```

As a result, any operator who configures a custom permission claim name finds:

1. **Users who should be blocked still get logon.** If the intended custom claim
   would have produced no Ego permissions for certain JWT holders (external
   accounts, low-privilege IdP users), they silently receive `ego.logon` and can
   authenticate to the Ego server.
2. **Elevated permissions are silently dropped.** Users whose custom claim would
   have mapped to `ego.root` or table-write permissions only receive logon; admin
   operations fail without a clear error.
3. **No warning or error is generated.** The misconfiguration is invisible in
   logs until an operator notices that elevated operations fail for users who
   should have admin access.

**Recommendation:**
At startup, when `ego.server.oauth.permission.claim` is set to a value other than
`"scope"` or `"roles"`, emit a SERVER-level warning (analogous to OAUTH-M1's
audience warning):

```go
if cfg.PermissionClaim != "scope" && cfg.PermissionClaim != "roles" {
    ui.Log(ui.ServerLogger, "oauth.rs.unsupported.permission.claim",
        ui.A{"claim": cfg.PermissionClaim})
}
```

Longer-term, support custom claim lookup by removing the `json:"-"` tag from
`jwtClaims.AdditionalClaims`, or by switching the claims struct to embed
`jwt.MapClaims` for the non-registered fields.

---

### Low / Informational (OAuth RS — second audit, June 2026)

#### OAUTH-L3 — `EGO_OAUTH_CLIENT_SECRET` environment variable not cleared after reading

**Affected file:** `server/oauth/config.go:130` — `loadConfig()`

```go
if envSecret := os.Getenv("EGO_OAUTH_CLIENT_SECRET"); envSecret != "" {
    clientSecret = envSecret
}
```

**Description:**
The OAuth2 client secret can be supplied via the `EGO_OAUTH_CLIENT_SECRET`
environment variable. Unlike `EGO_PASSWORD`, which was fixed by LOGIN-L1 to call
`os.Unsetenv` and emit a visible warning immediately after reading, the OAuth
client secret is read and stored but the environment variable is never cleared.
The secret therefore remains accessible to any child process spawned after server
startup (for example, via Ego's built-in exec functions if `ExecPermittedSetting`
is enabled) and is visible in `/proc/<pid>/environ` on Linux for the lifetime of
the server process.

**Recommendation:**
Clear the environment variable and emit a visible warning immediately after
reading, matching the pattern from LOGIN-L1:

```go
if envSecret := os.Getenv("EGO_OAUTH_CLIENT_SECRET"); envSecret != "" {
    clientSecret = envSecret
    os.Unsetenv("EGO_OAUTH_CLIENT_SECRET")
    ui.Log(ui.ServerLogger, "oauth.rs.client.secret.env", ui.A{})
}
```

---

#### OAUTH-L4 — Internal error details from token exchange and JWT validation returned to browser clients

**Affected files:**

- `server/oauth/rshandlers/callback.go:55-65` — IdP error branch
- `server/oauth/rshandlers/callback.go:97-103` — token exchange failure
- `server/oauth/rshandlers/callback.go:109-111` — JWT validation failure

```go
return util.ErrorResponse(w, session.ID,
    "token exchange failed: "+err.Error(), http.StatusBadGateway)

return util.ErrorResponse(w, session.ID,
    "received JWT is invalid: "+err.Error(), http.StatusBadGateway)
```

**Description:**
Three error responses in `CallbackHandler` include verbatim Ego error strings in
the HTTP response body returned to the browser:

1. **Token exchange failure** — `err.Error()` from `ExchangeCode` can contain
   the IdP token endpoint URL and the IdP's own error response.
2. **JWT validation failure** — `err.Error()` from `ValidateJWT` can reveal
   which specific validation step failed (signature, expiry, issuer, audience,
   missing claim), aiding an attacker who is probing token validation behavior.
3. **IdP error** — the raw `error` and `error_description` query parameters
   from the IdP redirect are concatenated verbatim into the response body
   (`"IdP authorization error: " + idpError + ": " + desc`). An attacker who
   can craft a redirect to the callback endpoint with arbitrary query parameters
   can inject arbitrary text into the response body — and also into the server
   log (`"desc": desc`), which may corrupt structured log output if the value
   contains newline characters or JSON control sequences.

**Recommendation:**
Return a fixed, generic message to the browser for all three failure paths and
keep full error detail in the AUTH log only:

```go
ui.Log(ui.AuthLogger, "oauth.rs.callback.exchange.failed", ui.A{
    "session": session.ID, "error": err.Error(),
})
return util.ErrorResponse(w, session.ID,
    "OAuth2 login failed", http.StatusBadGateway)
```

For the IdP error branch, sanitize `error_description` (replace newlines and
non-printable characters) before writing it to the log.

---

#### OAUTH-L5 — Custom `ego.server.oauth.user.claim` values silently fall back to `sub`

**Affected file:** `server/oauth/oauth.go:329-344` — `extractUsername()`

```go
func extractUsername(claims *jwtClaims, userClaim string) string {
    switch userClaim {
    case "sub":   return claims.Subject
    case "email": ...
    case "preferred_username": ...
    }
    return claims.Subject   // ← silent fallback for any other configured value
}
```

**Description:**
`extractUsername` handles only three well-known claim names (`sub`, `email`,
`preferred_username`). If an operator sets `ego.server.oauth.user.claim` to any
other value — for example `"upn"` (Azure AD), `"login"` (GitHub),
`"unique_name"` (ADFS), or a provider-specific custom claim — the function
silently returns `claims.Subject` with no warning or error.

Depending on the IdP, `sub` is often an opaque UUID rather than a human-readable
account name. Consequences include:

- Ego usernames appear as UUIDs in audit logs, making security reviews difficult.
- Username-based access policies apply to UUIDs rather than the account names
  the operator intended to control.
- The misconfiguration is invisible until an operator compares login usernames
  against expected values.

This is the user-identity analogue of OAUTH-M8 (custom permission claim silently
ignored).

**Recommendation:**
Log a startup warning when `ego.server.oauth.user.claim` is set to a value that
`extractUsername` does not handle. Longer-term, support custom claim lookup by
populating `AdditionalClaims` from the JWT body (removing its `json:"-"` tag) so
arbitrary claim names can be read at runtime.

---

## Remediation Checklist<a name="checklist"></a>

Use this checklist to track progress as issues are resolved.

### Critical items

- [x] **LOGIN-C1** — Replace SHA-256 with bcrypt (cost ≥ 12) for password storage; implement on-login migration for existing hashes
- [x] **LOGIN-C2** — Implement per-username failed-attempt counter and temporary lockout on the login endpoint
- [x] **TABLES-C1** — Change `CommitHandler` guard from `len(parameters) != 0` to `!= 1`; matches `RollbackHandler` and prevents panic on `parameters[0]` with empty slice

### High items

- [x] **OAUTH-H1** — `AuthorizePostHandler` now calls `router.CheckRateLimit` / `RecordFailure` / `RecordSuccess`; locked accounts receive a re-rendered form with a lockout message via the new `reRenderWithError` helper; budget shared with the native auth path
- [x] **OAUTH-H2** — `JWTCacheEntry` gained a `JTI string` field; `ValidateJWT` checks `tokens.IsIDBlacklisted(entry.JTI)` on every cache hit, evicts and rejects on a positive match, and stores `JTI: claims.ID` when writing new entries
- [x] **OAUTH-H3** — `handleAuthorizationCodeGrant` now rejects exchanges where `client.ClientSecretHash == ""` and `pending.CodeChallenge == ""`; new i18n keys `oauth.as.pkce.required` and `oauth.as.pkce.missing` added to all three language files
- [x] **OAUTH-H4** — All failure re-render paths in `AuthorizePostHandler` route through the new `reRenderWithError` helper, which always generates a fresh CSRF token, replaces the cookie, and populates `loginFormData.CSRFToken`
- [x] **OAUTH-H5** — Removed unvalidated `redirect` query-parameter override from `AuthorizeRedirectHandler` and `.Parameter("redirect")` from route registration; removed dead `RedirectURI` field from `pendingState`/`PendingState` and the `newState()` parameter; tests in `rshandlers/authorize_handler_test.go`
- [x] **LOGIN-H1** — Replace `==` password comparison with `crypto/subtle.ConstantTimeCompare`
- [x] **LOGIN-H2** — Upgraded in two stages: (1) MD5 → PBKDF2-SHA256 in `util/crypto.go`; (2) both `util/crypto.go` and `app-cli/settings/crypto.go` upgraded to Argon2id (32 MiB, 2 iterations) with per-encryption random salt and `ÿEG3` magic prefix; all existing ciphertext decrypts transparently via legacy paths
- [x] **LOGIN-H3** — Token key already stored in AES-256-GCM encrypted sidecar file by settings infrastructure; not in plaintext profile JSON
- [x] **LOGIN-H4** — Redirect following disabled on logon POST; 3xx responses return an error telling the user to update their server URL
- [x] **WEBAUTH-H1** — `passkeyGuard()` added; all five ceremony handlers return 404 when `ego.server.allow.passkeys` is false
- [x] **WEBAUTH-H2** — `credential.Authenticator.CloneWarning` checked after `FinishDiscoverableLogin`; login rejected with 401 on clone detection
- [x] **HTTP-H1** — `http.MaxBytesReader` wraps `r.Body` before `io.ReadAll`; returns 413 on oversize body; limit defaults to 32 MiB, configurable via `ego.server.max.body.size`
- [x] **HTTP-H2** — `makeHTTPServer()` helper constructs `http.Server` with `ReadHeaderTimeout` (10 s), `ReadTimeout` (30 s), `WriteTimeout` (120 s), and `IdleTimeout` (120 s); all three listeners (plain HTTP, TLS, and HTTP→HTTPS redirect) use it; all four values are configurable via `ego.server.{read.header|read|write|idle}.timeout`
- [x] **ASSET-H1** — `end` clamped to `totalSize - 1` in `readAssetRange` after `os.Stat`; `size` computed as `end - start + 1` (inclusive); eliminates the ~9 EB `make` allocation triggered by open-ended Range headers
- [x] **PROFILE-H1** — `encrypt`/`decrypt` in `app-cli/settings/crypto.go` now use `sha256.Sum256` for key derivation; new ciphertext carries a `"v2:"` prefix; `Decrypt` falls back to legacy MD5 path for prefix-less values
- [x] **TABLES-H1** — Route schema substitution in `listTables` through `parsing.QueryParameters` (which calls `SQLEscape`) instead of bare `strings.ReplaceAll`
- [x] **TABLES-H2** — Add missing `!` to `Authorized` call in `getTableNames` so tables the user cannot read are filtered out, not those they can
- [x] **CODE-H1** — `Sandboxed()` in `bytecode/context.go` sets `sandboxedExec = false` when `flag` is true, blocking subprocess exec in all sandboxed admin/run contexts unconditionally, even when `ExecPermittedSetting` is enabled globally

### Medium items

- [x] **OAUTH-M1** — After successful `oauth.Initialize()`, `commands/server.go` checks `oauth.GetConfig().Audience == ""` and emits a SERVER-level `oauth.rs.no.audience` log warning; key added to all three language files
- [x] **OAUTH-M2** — `server/oauth/client.go` defines `idpClient = &http.Client{Timeout: 10 * time.Second}`; all three server-side outbound call sites use it; CLI gains `oauthHTTPClient` (30 s) for `fetchOIDCDiscovery` and `postTokenRequest`
- [x] **OAUTH-M3** — `RevokeHandler` in `revoke.go` now calls `validateBasicAuth(r)` instead of two `r.FormValue` calls; accepts Basic Auth header and form fields; backward compatible
- [x] **OAUTH-M4** — Both `discoverEndpoints` and `refreshJWKS` now use `&io.LimitedReader{N: 1<<20 + 1}` before `io.ReadAll`; `lr.N == 0` after reading indicates an oversized body and returns a descriptive error
- [x] **OAUTH-M5** — `newState()` checks `len(stateStore.items) >= maxPendingStates` (500) inside the same mutex lock as the insert; `statePurgeInterval = 2 * time.Minute` constant replaces `stateMaxAge` in the purge ticker
- [ ] **OAUTH-M6** — Wrap `resp.Body` in `io.LimitedReader` (64 KiB) before `io.ReadAll` in `ExchangeCode` (`flow_authcode.go:155`), matching the pattern applied to discovery and JWKS responses by OAUTH-M4
- [ ] **OAUTH-M7** — Add a `lastMissRefresh` cooldown in `keyByID` (`jwks.go`) so that a fresh-cache unknown-`kid` triggers at most one JWKS refresh per 30-second window, preventing a JWKS storm from tokens with unique fake key IDs
- [ ] **OAUTH-M8** — Emit a startup warning when `ego.server.oauth.permission.claim` is set to a value other than `"scope"` or `"roles"`, since custom claim names are silently unsupported and all JWT holders fall back to `"ego.logon"`
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
- [x] **PROFILE-M1** — Both read paths (`readOutboardConfigFiles`, `Load`) now call `NeedsNewHash()` after decrypt and immediately write back with the SHA-256 scheme; file path uses `os.WriteFile`, database path uses `UPDATE` after cursor close
- [x] **TABLES-M1** — Replace `"DROP TABLE " + tableName` in the DSN branch of `DeleteTable` with a quoted identifier; route through `parsing.QueryParameters` for consistency
- [x] **TABLES-M2** — Convert row ID filter in `formAbstractUpdateQuery` to a `$N` numbered parameter passed to `db.Exec` rather than string-embedded in the WHERE clause
- [x] **TABLES-M3** — Replace `defs.AdminAgent` with `defs.TableAdminPermission` in the `DeleteTable` authorization check; correct the error message from "read permission" to "admin permission"
- [x] **TABLES-M4** — Enforce a server-side maximum row limit in `PagingClauses`; default to 1000 rows when no limit is specified
- [x] **CODE-M1** — `RunCodeHandler` wraps `r.Body` with `http.MaxBytesReader` (256 KiB); `*http.MaxBytesError` returns 413; post-decode `len(req.Code)` check also returns 413
- [x] **CODE-M2** — Save/restore of global `TraceLogger` replaced with `traceRunMu` mutex held for the duration of trace-enabled executions; non-trace requests are not serialized
- [x] **CODE-M3** — `uuid.Parse` validates `Session` field before use; `owner` field added to `codeSessionEntry` and `debugSession`; ownership enforced on every session lookup with an opaque error; `run.not.found` key added to all three language files
- [ ] **CODE-M4** — Resolve symlinks via `filepath.EvalSymlinks` in `sandboxName` and verify the resolved path remains inside the sandbox root before returning it
- [ ] **CODE-M5** — Audit all `ExtensionsEnabledSetting`-gated behavior for sandbox compatibility; disable extension features that conflict with `Sandboxed(true)` constraints

### Low / Informational items

- [x] **OAUTH-L1** — `isSecureRequest` renamed to `IsSecureRequest` (exported) in `router/webauthn.go`; both cookie-set sites in `authorize.go` (`AuthorizeGetHandler` and `reRenderWithError`) now pass `Secure: router.IsSecureRequest(r)`; tests in `low_test.go` and `secure_request_test.go`
- [x] **OAUTH-L2** — `.AcceptMedia(defs.JSONMediaType)` removed from the `POST /oauth2/token` route in `RegisterRoutes`; no Accept-header constraint is imposed; tests in `low_test.go` confirm the handler processes form-encoded requests without an Accept header
- [ ] **OAUTH-L3** — Call `os.Unsetenv("EGO_OAUTH_CLIENT_SECRET")` and emit a log warning immediately after reading the env var in `loadConfig` (`config.go`), matching the pattern from LOGIN-L1
- [ ] **OAUTH-L4** — Return a fixed generic message to the browser for all three failure paths in `CallbackHandler`; keep full error detail in the AUTH log; sanitize `error_description` before logging to prevent log injection
- [ ] **OAUTH-L5** — Emit a startup warning when `ego.server.oauth.user.claim` is set to a value that `extractUsername` does not handle (`sub`, `email`, `preferred_username` are the only supported values)
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
- [x] **TABLES-L1** — Log full database errors server-side and return generic messages in HTTP error responses from the tables package
- [x] **TABLES-L2** — Wrap table and index names in double-quotes in all three SQLite `PRAGMA` format strings in `describe.go`
- [x] **CODE-L1** — `RunCodeHandler` copies request to `logReq` and truncates `logReq.Code` to 120 characters before `json.MarshalIndent`; original `req.Code` is unmodified
