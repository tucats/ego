# LOGIN-M1 — HTTP downgrade when HTTPS connection fails

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

**Resolution:**  
HTTP fallback removed from `resolveServerName`; unqualified names only try
HTTPS. Explicit `http://` scheme still accepted as the user's deliberate
choice.

