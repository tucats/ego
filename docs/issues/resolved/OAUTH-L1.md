# OAUTH-L1 — CSRF cookie missing `Secure` flag on AS authorize handler

**Affected file:** `server/oauth/authserver/authorize.go:147` — `AuthorizeGetHandler()`

```go
http.SetCookie(w, &http.Cookie{
    Name:     csrfCookieName,
    Value:    csrfToken,
    Path:     "/oauth2/authorize",
    HttpOnly: true,
    SameSite: http.SameSiteStrictMode,
    // Secure is not set
})
```

**Description:**
The CSRF cookie used to protect the AS login form is `HttpOnly` and
`SameSite: Strict`, but the `Secure` attribute is not set. In any configuration
where the Ego AS accepts plain HTTP connections (before HTTPS redirect, or in
development), the CSRF nonce can be transmitted in cleartext. A network observer
can capture the cookie and the form nonce, enabling CSRF attacks against users on
unencrypted connections.

This is the same issue as WEBAUTH-L1, which was already resolved for the WebAuthn
challenge cookie. The `isSecureRequest(r)` helper added there can be reused here.

**Recommendation:**
Set the `Secure` attribute conditionally using `isSecureRequest(r)`:

```go
http.SetCookie(w, &http.Cookie{
    Name:     csrfCookieName,
    Value:    csrfToken,
    Path:     "/oauth2/authorize",
    HttpOnly: true,
    SameSite: http.SameSiteStrictMode,
    Secure:   isSecureRequest(r),
})
```

Apply the same fix to the re-render path in `AuthorizePostHandler` once
OAUTH-H4 is resolved and a new CSRF token is generated there as well.

**Resolution (May 2026):**
`isSecureRequest` in `router/webauthn.go` was renamed to `IsSecureRequest`
(exported) so that the `authserver` package can call it without duplicating the
logic.  All four internal call sites in `router/webauthn.go` and all three
references in `router/webauthn_test.go` were updated to use the new name.
Both cookie-setting locations in `authorize.go` — `AuthorizeGetHandler` and
`reRenderWithError` — now pass `Secure: router.IsSecureRequest(r)`.  The flag
is `true` when `r.TLS != nil` (direct TLS) or when the `X-Forwarded-Proto:
https` header is set (proxy-terminated TLS); it is `false` for plain HTTP so
development servers are not broken.  Tests in
`server/oauth/authserver/low_test.go` and
`server/oauth/authserver/secure_request_test.go` cover: HTTPS sets Secure,
plain HTTP omits Secure, X-Forwarded-Proto sets Secure, and
`router.IsSecureRequest` is callable from outside the router package.

