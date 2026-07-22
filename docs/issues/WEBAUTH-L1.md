# WEBAUTH-L1 — `Secure` flag absent on challenge cookie

**Affected file:** `server/server/webauthn.go:51` — `challengeCookie()`

**Description:**  
The nonce cookie used to correlate the browser with the server's challenge cache
is `HttpOnly` and `SameSite: Strict`, but the `Secure` attribute is not set.
In any configuration where the server accepts plain HTTP (before an HTTPS
redirect, or in a development setup), the nonce could be transmitted in
cleartext. Browsers enforce HTTPS for WebAuthn themselves, which limits
practical exposure in the field, but the cookie hardening is incomplete.

**Resolution:**  
`challengeCookie` now accepts a `secure bool` parameter. The `isSecureRequest(r)`
helper returns `true` when `r.TLS != nil` or `X-Forwarded-Proto: https` is set.
All four call sites pass `isSecureRequest(r)`.

