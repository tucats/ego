# LOGIN-M4 — Quoted-password legacy format allows plaintext storage

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

**Resolution:**  
`{quoted}` format now logs an `auth.password.plaintext` warning on every use;
bcrypt migration on first successful login was already in place from
LOGIN-C1. Remove the special-case block once no `{quoted}` entries remain in
the user database.

