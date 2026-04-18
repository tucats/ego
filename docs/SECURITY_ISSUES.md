# Security Issues — Authentication & Login

This document records known security weaknesses in the authentication and login
subsystem identified during a code review in April 2026. It is intended as a
living reference for future developers: each section describes the risk, the
affected code, and a concrete recommendation. A checklist at the bottom tracks
remediation progress.

---

## Critical

### C1 — Weak password hashing (SHA-256, no salt)

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

### C2 — No brute-force or rate-limiting protection on the login endpoint

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

## High

### H1 — Timing attack in password comparison

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

### H2 — MD5 used to derive AES encryption keys

**Affected file:** `util/crypto.go:56` — `encrypt()` / `decrypt()`

```go
block, _ := aes.NewCipher([]byte(Hash(passphrase)))
// Hash() uses crypto/md5
```

**Description:**  
`util.Hash()` uses MD5, which produces a 128-bit (16-byte) output. AES-256
requires a 32-byte key; passing 16 bytes will cause `aes.NewCipher` to return
an error (silently discarded with `_`), meaning encryption may silently fail or
fall back to AES-128 depending on how the error propagates. Beyond the
key-length issue, MD5 is cryptographically broken and must not be used for
any security-sensitive derivation. This affects the optional encrypted user
file and all token encryption.

**Recommendation:**  
At minimum, replace `Hash()` in `encrypt()`/`decrypt()` with
`sha256.Sum256([]byte(passphrase))[:]` to get a correct 32-byte key and
eliminate MD5. For production-grade key derivation, use PBKDF2
(`golang.org/x/crypto/pbkdf2`) or HKDF with a stored random salt.

---

### H3 — Token signing key stored in plaintext configuration

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

### H4 — Login credentials forwarded on HTTP 301 redirect to arbitrary host

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

## Medium

### M1 — HTTP downgrade when HTTPS connection fails

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

### M2 — Password whitespace stripped before hashing

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

### M3 — Token cache bypasses expiry and revocation checks

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

### M4 — Quoted-password legacy format allows plaintext storage

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

## Low / Informational

### L1 — Password supplied via environment variable

**Affected file:** `app-cli/app/logon.go:38` — `EgoPasswordEnv`

Environment variables are visible to all processes owned by the same user and
are inherited by child processes. Using `EGO_PASSWORD` to supply credentials is
a common CI convenience but should be noted in any threat model. Consider
supporting a credentials file with `0600` permissions or a secrets-manager
integration as a more secure alternative for automated contexts.

---

### L2 — `InsecureSkipVerify` available without prominent warning

TLS certificate verification can be disabled for self-signed certificates.
In development this is acceptable, but the flag should trigger a visible
warning in production-like configurations and should not be silently inherited
by default from a stored profile setting.

---

### L3 — Returned token expiration recalculated independently of token contents

**Affected file:** `server/server/admin.go:99`

The expiration timestamp returned in the logon response body is computed from
the server's current max-expiration setting at response time, independently of
the expiry embedded inside the encrypted token. If the server's configured
maximum token duration changes between token issuance and token use, the
advisory expiration seen by the client and the enforced expiry inside the token
can diverge. This does not affect security directly but can cause confusing
"token expired" errors before the displayed expiry has passed.

---

---

## Security Issues — WebAuthn / Passkeys

This section records security weaknesses in the WebAuthn passkey subsystem
identified during a code review in April 2026. Issues are rated using the same
severity scale as the authentication section above.

---

### High (WebAuthn)

#### WA-H1 — `allow.passkeys` setting not enforced in ceremony handlers

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

#### WA-H2 — Authenticator clone warning not checked after login

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

#### WA-M1 — No rate limiting on unauthenticated ceremony-begin endpoints

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

#### WA-M2 — RPID derived from user-controlled `Host` header

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

#### WA-M3 — `storeChallenge` mutates shared cache expiration on every call

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

#### WA-L1 — `Secure` flag absent on challenge cookie

**Affected file:** `server/server/webauthn.go:51` — `challengeCookie()`

**Description:**  
The nonce cookie used to correlate the browser with the server's challenge cache
is `HttpOnly` and `SameSite: Strict`, but the `Secure` attribute is not set.
In any configuration where the server accepts plain HTTP (before an HTTPS
redirect, or in a development setup), the nonce could be transmitted in
cleartext. Browsers enforce HTTPS for WebAuthn themselves, which limits
practical exposure in the field, but the cookie hardening is incomplete.

**Recommendation:**  
Set `Secure: true` when the connection is TLS or a trusted forwarded-proto
header indicates HTTPS:

```go
Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
```

---

#### WA-L2 — Cache item expiration only refreshed when CACHE logging is active

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

**Recommendation:**  
Move the expiration-refresh update outside the logging block so it always
executes on a cache hit, regardless of log level. Keep the log call inside the
block.

---

#### WA-L3 — No user notification when passkeys are cleared by an administrator

**Affected file:** `server/server/webauthn.go` — `WebAuthnClearPasskeysHandler`

**Description:**  
An administrator can silently remove all passkeys for any user account. The
operation is audit-logged server-side, but the affected user receives no
in-band notification. A compromised admin account could degrade every user's
authentication to password-only without any visible indication to those users.

**Recommendation:**  
Consider sending an in-band notification (e.g. a log entry visible in the
dashboard, or an email if the server supports it) to the affected user when
their passkeys are cleared by someone other than themselves.

---

## Remediation Checklist

Use this checklist to track progress as issues are resolved. Update the item
with the commit hash or PR reference when closed.

### Critical items

- [x] **C1** — Replace SHA-256 with bcrypt (cost ≥ 12) for password storage; implement on-login migration for existing hashes
- [x] **C2** — Implement per-username failed-attempt counter and temporary lockout on the login endpoint

### High items

- [x] **H1** — Replace `==` password comparison with `crypto/subtle.ConstantTimeCompare`
- [x] **H2** — Replace MD5 key derivation in `util/crypto.go` with SHA-256 or PBKDF2
- [x] **H3** — Token key already stored in AES-256-GCM encrypted sidecar file by settings infrastructure; not in plaintext profile JSON
- [x] **H4** — Redirect following disabled on logon POST; 3xx responses return an error telling the user to update their server URL

### Medium items

- [x] **M1** — HTTP fallback removed from `resolveServerName`; unqualified names only try HTTPS. Explicit `http://` scheme still accepted as the user's deliberate choice.
- [x] **M2** — Removed `strings.TrimSpace` from password handling; prompt loop now uses `pass == ""` so spaces-only passwords are accepted as-is
- [x] **M3** — Cache now stores `*tokens.Token`; cache hits check `Expires` directly (no re-decryption). Blacklist is already handled: `tokens.Blacklist()` purges the token cache at revocation time.
- [x] **M4** — `{quoted}` format now logs an `auth.password.plaintext` warning on every use; bcrypt migration on first successful login was already in place from C1. Remove the special-case block once no `{quoted}` entries remain in the user database.

### Low / Informational items

- [ ] **L1** — Document `EGO_PASSWORD` env var risk; consider supporting a `0600` credentials file for CI use
- [ ] **L2** — Print a visible warning when `InsecureSkipVerify` is active
- [ ] **L3** — Derive the advisory expiration from the token's embedded expiry rather than recalculating it independently

### WebAuthn high items

- [x] **WA-H1** — `passkeyGuard()` added; all five ceremony handlers return 404 when `ego.server.allow.passkeys` is false
- [x] **WA-H2** — `credential.Authenticator.CloneWarning` checked after `FinishDiscoverableLogin`; login rejected with 401 on clone detection

### WebAuthn medium items

- [x] **WA-M1** — `webAuthnBeginGuard()` added: per-IP sliding-window rate limit (10 req/min) and global pending-ceremony cap (200) enforced on both begin endpoints; returns 429 on breach
- [x] **WA-M2** — Startup warning emitted via `server.webauthn.no.rpid` log key when passkeys are enabled but `ego.server.webauthn.rpid` is not configured
- [x] **WA-M3** — `caches.SetExpiration` moved from `storeChallenge` to `defineNativeAdminHandlers` (server startup); called exactly once

### WebAuthn low / informational items

- [ ] **WA-L1** — Set `Secure: true` on the challenge cookie when the connection is TLS or `X-Forwarded-Proto: https`
- [ ] **WA-L2** — Move cache expiration refresh in `caches/find.go` outside the `ui.IsActive(ui.CacheLogger)` block
- [ ] **WA-L3** — Notify affected users (via dashboard or log) when an administrator clears their passkeys
