# HTTP-M1 — Security response headers not set

**Affected file:** `server/server/serve.go` — `ServeHTTP()`

**Description:**  
The server never adds any of the standard browser-security response headers.
While most Ego server clients are CLI tools or the dashboard SPA rather than
general browsers, the dashboard is a browser application and omitting these
headers leaves it exposed to well-known classes of attack:

| Missing header | Risk if absent |
| :--- | :--- |
| `Content-Security-Policy` | XSS via injected scripts in the dashboard |
| `X-Content-Type-Options` (no-sniff) | MIME-sniffing attacks on API responses |
| `X-Frame-Options: DENY` | Click-jacking via embedding the dashboard in an iframe |
| `Strict-Transport-Security` | Browser allows downgrade to HTTP after first HTTPS visit |
| `Referrer-Policy` | Auth tokens in URL leaked via the HTTP referrer request header |

**Recommendation:**  
Add a thin middleware layer (or add directly to `ServeHTTP` before calling the
handler) that sets the defensive headers on every response:

```go
w.Header().Set("X-Content-Type-Options", "nosniff")
w.Header().Set("X-Frame-Options", "DENY")
w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
if r.TLS != nil {
    w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
}
```

A `Content-Security-Policy` appropriate for the dashboard requires more
thought (it must whitelist the sources used by the dashboard's JavaScript and
CSS), but at minimum a restrictive default of `default-src 'self'` should be
set and relaxed only for the dashboard routes that need it.

Due to dashboard problems, current CSP settings are as follows. The unsafe-inline
was added because dashboard was being blocked from itself.

- default-src 'self'
- script-src 'self' 'unsafe-inline'
- style-src 'self' 'unsafe-inline'
- object-src 'none'
- base-uri 'self'

**Resolution:**  
`addSecurityHeaders()` in `serve.go` sets `X-Content-Type-Options`,
`X-Frame-Options`, `Referrer-Policy`, `Content-Security-Policy`, and (TLS only)
`Strict-Transport-Security` on every response.

