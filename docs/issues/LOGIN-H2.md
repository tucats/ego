# LOGIN-H2 — Weak key derivation in AES encryption

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

