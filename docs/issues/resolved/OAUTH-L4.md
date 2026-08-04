# OAUTH-L4 — Internal error details from token exchange and JWT validation returned to browser clients

**Affected files:**

- `server/oauth/rshandlers/callback.go:55-65` — IdP error branch
- `server/oauth/rshandlers/callback.go:97-103` — token exchange failure
- `server/oauth/rshandlers/callback.go:109-111` — JWT validation failure

```go
return util.ErrorResponse(w, session.ID,
    "token exchange failed: "+err.Error(), http.StatusBadGateway)

return util.ErrorResponse(w, session.ID,
    "received JWT is invalid: "+err.Error(), http.StatusBadGateway)
```

**Description:**
Three error responses in `CallbackHandler` include verbatim Ego error strings in
the HTTP response body returned to the browser:

1. **Token exchange failure** — `err.Error()` from `ExchangeCode` can contain
   the IdP token endpoint URL and the IdP's own error response.
2. **JWT validation failure** — `err.Error()` from `ValidateJWT` can reveal
   which specific validation step failed (signature, expiry, issuer, audience,
   missing claim), aiding an attacker who is probing token validation behavior.
3. **IdP error** — the raw `error` and `error_description` query parameters
   from the IdP redirect are concatenated verbatim into the response body
   (`"IdP authorization error: " + idpError + ": " + desc`). An attacker who
   can craft a redirect to the callback endpoint with arbitrary query parameters
   can inject arbitrary text into the response body — and also into the server
   log (`"desc": desc`), which may corrupt structured log output if the value
   contains newline characters or JSON control sequences.

**Recommendation:**
Return a fixed, generic message to the browser for all three failure paths and
keep full error detail in the AUTH log only:

```go
ui.Log(ui.AuthLogger, "oauth.rs.callback.exchange.failed", ui.A{
    "session": session.ID, "error": err.Error(),
})
return util.ErrorResponse(w, session.ID,
    "OAuth2 login failed", http.StatusBadGateway)
```

For the IdP error branch, sanitize `error_description` (replace newlines and
non-printable characters) before writing it to the log.

**Resolution (June 2026):**
Four changes were made to `server/oauth/rshandlers/callback.go`:

1. **`sanitizeLogValue(s string) string`** — new unexported helper added before
   `CallbackHandler`.  It iterates over the runes in `s` and replaces any
   character where `unicode.IsControl(r)` is true with a space, then trims
   leading and trailing spaces with `strings.TrimSpace`.  `unicode.IsControl`
   covers the full Unicode Cc category (U+0000–U+001F and U+007F–U+009F),
   which includes all ASCII control codes including `\n`, `\r`, `\t`, and NUL.
   Non-ASCII printable Unicode (accented letters, CJK, emoji) is passed through
   unchanged.  The `strings` and `unicode` packages were added to the import
   block.

2. **IdP error branch** — both `idpError` and `desc` are now passed through
   `sanitizeLogValue` before being written to the AUTH log under
   `oauth.rs.callback.idp.error`.  The browser response was changed from
   `"IdP authorization error: "+idpError+": "+desc` to the fixed string
   `"OAuth2 login failed"`.

3. **Token exchange failure** — the browser response was changed from
   `"token exchange failed: "+err.Error()` to `"OAuth2 login failed"`.  The
   existing AUTH log entry (under `oauth.rs.callback.exchange.failed`) is
   unchanged and still carries the full error detail.

4. **JWT validation failure** — the browser response was changed from
   `"received JWT is invalid: "+err.Error()` to `"OAuth2 login failed"`.
   `ValidateJWT` already logs the failure internally under `oauth.rs.jwt.invalid`,
   so no additional log entry was needed here.

Tests added:

- `callback_test.go` (external `package rshandlers_test`) — three tests verify
  that the response body is the fixed generic message and never contains the raw
  IdP error codes, server addresses, or injected newline sequences.
- `sanitize_test.go` (internal `package rshandlers`) — `TestSanitizeLogValue`
  with 13 sub-cases directly exercises `sanitizeLogValue`, covering plain ASCII,
  newline, CR, CRLF, tab, NUL, DEL, leading/trailing whitespace trimming, non-ASCII
  Unicode passthrough, and the all-control-chars edge case.

