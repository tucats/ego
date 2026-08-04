# LOGIN-L3 — Returned token expiration recalculated independently of token contents

**Affected file:** `server/server/admin.go:99`

The expiration timestamp returned in the logon response body is computed from
the server's current max-expiration setting at response time, independently of
the expiry embedded inside the encrypted token. If the server's configured
maximum token duration changes between token issuance and token use, the
advisory expiration seen by the client and the enforced expiry inside the token
can diverge. This does not affect security directly but can cause confusing
"token expired" errors before the displayed expiry has passed.

**Resolution (April 2026):**  
`LogonHandler` in `server/server/admin.go` now calls `tokens.Unwrap` on the
freshly-minted token string immediately after creating it. `response.Expiration`
is set from `t.Expires.Format(time.UnixDate)` — the value baked into the token
itself — and the previous independent duration re-calculation block has been
removed. The `tokens.Unwrap` call also surfaces the `TokenID` needed for
`response.ID`, consolidating two previously separate concerns into one call.

