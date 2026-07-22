# OAUTH-L2 — AS token endpoint router registration uses JSON content type

**Affected file:** `server/oauth/authserver/authserver.go:109` — `RegisterRoutes()`

```go
r.New(defs.OAuthTokenPath, TokenHandler, http.MethodPost).
    Class(router.ServiceRequestCounter).
    AcceptMedia(defs.JSONMediaType)
```

**Description:**
The token endpoint (`POST /oauth2/token`) is registered in the router with
`AcceptMedia(defs.JSONMediaType)` (i.e., `application/json`). However, RFC 6749
§4.1.3 and §4.4.2 require clients to send token requests as
`application/x-www-form-urlencoded`. The handler already uses `r.ParseForm()` to
read the request body, which is consistent with form encoding, not JSON.

A strictly compliant OAuth2 client library that sets
`Content-Type: application/x-www-form-urlencoded` (as required) may be rejected
by the Ego router before `TokenHandler` is ever called, if the router enforces
the declared `AcceptMedia` type. The `Authorization Code` flow from the CLI is
unaffected in practice because the CLI's `postTokenRequest` sets
`Content-Type: application/x-www-form-urlencoded` and the router may be lenient
in practice, but third-party clients that rely on strict content-type enforcement
(e.g., `application/json`) on the wrong side of the mismatch will fail.

**Recommendation:**
Remove the `AcceptMedia(defs.JSONMediaType)` chain call from the token endpoint
registration, or replace it with the correct media type:

```go
r.New(defs.OAuthTokenPath, TokenHandler, http.MethodPost).
    Class(router.ServiceRequestCounter)
    // No AcceptMedia constraint — RFC 6749 requires form-encoded bodies
```

**Resolution (May 2026):**
The `.AcceptMedia(defs.JSONMediaType)` chain call was removed from the
`POST /oauth2/token` route registration in `RegisterRoutes`
(`server/oauth/authserver/authserver.go`).  The router now imposes no
restriction on the Accept header for this endpoint, matching the RFC 6749
requirement that clients send `application/x-www-form-urlencoded` request
bodies.  The handler (`TokenHandler`) uses `r.ParseForm()` internally and
always writes JSON responses, regardless of what the client declares in its
Accept header.  Tests in `server/oauth/authserver/low_test.go` verify that
the handler processes form-encoded requests without an Accept header and that
the error path (unsupported grant type) produces a grant-type error rather
than a media-type rejection.

