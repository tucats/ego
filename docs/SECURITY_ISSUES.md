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

## Remediation Checklist

Use this checklist to track progress as issues are resolved. Update the item
with the commit hash or PR reference when closed.

### Critical

- [x] **C1** — Replace SHA-256 with bcrypt (cost ≥ 12) for password storage; implement on-login migration for existing hashes
- [ ] **C2** — Implement per-username failed-attempt counter and temporary lockout on the login endpoint

### High

- [ ] **H1** — Replace `==` password comparison with `crypto/subtle.ConstantTimeCompare`
- [ ] **H2** — Replace MD5 key derivation in `util/crypto.go` with SHA-256 or PBKDF2
- [ ] **H3** — Remove token signing key from plaintext config; require secure key provisioning at server startup
- [ ] **H4** — Stop forwarding credentials on HTTP 301 redirects to unverified hosts

### Medium

- [ ] **M1** — Remove silent HTTP downgrade fallback from `resolveServerName`; require explicit opt-in for plain HTTP
- [ ] **M2** — Remove `strings.TrimSpace` from password handling in `Logon`
- [ ] **M3** — Recheck token expiry and blacklist status even on cache hit
- [ ] **M4** — Convert `{plaintext}` legacy password format to a one-time migration; remove special-case logic once migration is complete

### Low / Informational

- [ ] **L1** — Document `EGO_PASSWORD` env var risk; consider supporting a `0600` credentials file for CI use
- [ ] **L2** — Print a visible warning when `InsecureSkipVerify` is active
- [ ] **L3** — Derive the advisory expiration from the token's embedded expiry rather than recalculating it independently
