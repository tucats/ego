# PROFILE-M1 — Legacy ciphertext not re-encrypted on successful read

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

