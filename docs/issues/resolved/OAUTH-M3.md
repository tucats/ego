# OAUTH-M3 — Revocation endpoint ignores HTTP Basic Auth for client authentication

**Affected file:** `server/oauth/authserver/revoke.go:31` — `RevokeHandler()`

```go
clientID := r.FormValue("client_id")
clientSecret := r.FormValue("client_secret")
```

**Description:**
The AS token endpoint (`handleAuthorizationCodeGrant`, `handleClientCredentialsGrant`,
`handleRefreshTokenGrant`) uses `validateBasicAuth(r)`, which prefers the HTTP
`Authorization: Basic` header and falls back to form-encoded `client_id` /
`client_secret` fields. This matches RFC 6749 §2.3.1.

The revocation endpoint (`POST /oauth2/revoke`) reads credentials only from form
values, completely ignoring any `Authorization: Basic` header. RFC 7009 §2.1
requires the revocation endpoint to use the same client authentication mechanism
as the token endpoint. Confidential clients that prefer Basic Auth cannot
authenticate at the revocation endpoint and will receive a 401 error even when
supplying valid credentials.

**Recommendation:**
Replace the two `r.FormValue` calls with a call to the existing `validateBasicAuth`
helper:

```go
clientID, clientSecret := validateBasicAuth(r)
```

This is a one-line fix that brings the revocation endpoint into alignment with
the token endpoint and RFC 7009.

**Resolution (May 2026):**
The two `r.FormValue("client_id")` / `r.FormValue("client_secret")` lines in
`RevokeHandler` (`server/oauth/authserver/revoke.go`) were replaced with a
single call to the existing `validateBasicAuth(r)` helper (defined in
`token.go`).  `validateBasicAuth` tries the `Authorization: Basic` header first
and falls back to form fields, so all existing clients that POST credentials
in the body continue to work without changes.  Tests in
`server/oauth/authserver/revoke_test.go` cover: Basic Auth accepted, form
credentials still accepted, wrong secret rejected, unknown client rejected, and
public-client Basic Auth with empty password accepted.

