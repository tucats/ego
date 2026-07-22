# LOGIN-C1 — Weak password hashing (SHA-256, no salt)

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

**Resolution:**  
Replace SHA-256 with bcrypt (cost ≥ 12) for password storage; implement
on-login migration for existing hashes.

