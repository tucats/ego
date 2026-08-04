# LOGIN-M2 — Password whitespace stripped before hashing

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

**Resolution:**  
Removed `strings.TrimSpace` from password handling; prompt loop now uses
`pass == ""` so spaces-only passwords are accepted as-is.

