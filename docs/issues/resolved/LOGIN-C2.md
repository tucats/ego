# LOGIN-C2 — No brute-force or rate-limiting protection on the login endpoint

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

**Resolution:**  
Implement per-username failed-attempt counter and temporary lockout on the
login endpoint.

