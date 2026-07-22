# PROFILE-H1 — MD5 used to derive the AES encryption key

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

