# OAUTH-M6 — Token exchange response body read without size limit

**Affected file:** `server/oauth/flow_authcode.go:155` — `ExchangeCode()`

```go
body, err := io.ReadAll(resp.Body)
```

**Description:**
`ExchangeCode` reads the IdP token endpoint response with a bare `io.ReadAll`,
applying no byte limit before the allocation. OAUTH-M4 fixed the same class of
issue for the OIDC discovery document and JWKS responses, but the token exchange
response was not updated at the same time.

A malicious or compromised IdP token endpoint (or a network attacker who can
intercept the server-to-server TLS connection) can return an arbitrarily large
response body. The `idpClient` timeout (10 s, added by OAUTH-M2) bounds the
total wall-clock time, but a slowly-streaming attacker can deliver megabytes
within that window. The full body is allocated as a single byte slice, so a
large payload causes a memory spike that can be repeated on every user login that
triggers a token exchange (i.e., every Authorization Code flow completion).

**Recommendation:**
Apply the same `io.LimitedReader` pattern used in `discoverEndpoints` and
`refreshJWKS`:

```go
const maxTokenBodyBytes = 64 << 10  // 64 KiB — generous for any real token response
lr := &io.LimitedReader{R: resp.Body, N: maxTokenBodyBytes + 1}
body, err := io.ReadAll(lr)
if err != nil {
    return "", "", errors.New(errors.ErrOAuthTokenRead).Context(err.Error())
}
if lr.N == 0 {
    return "", "", errors.New(errors.ErrOAuthTokenSizeLimit).Context(doc.TokenEndpoint)
}
```

A real token endpoint response is a small JSON object (access token, expiry,
scopes) — 64 KiB is many times larger than any legitimate response.

**Resolution (June 2026):**
`ExchangeCode` in `server/oauth/flow_authcode.go` now wraps `resp.Body` in
`&io.LimitedReader{R: resp.Body, N: maxTokenBodyBytes + 1}` (where
`maxTokenBodyBytes = 64 << 10`, 64 KiB) before calling `io.ReadAll`.  Using
N+1 rather than N avoids an off-by-one: when `lr.N` reaches zero after reading,
it proves the body is strictly larger than the limit rather than exactly equal
to it.  A zero `lr.N` after `io.ReadAll` returns causes `ExchangeCode` to
return the new `ErrOAuthTokenSizeLimit` error so operators can identify why a
token exchange was rejected.  New error constant `ErrOAuthTokenSizeLimit` added
to `errors/messages.go` with key `oauth.token.size`; the localized message was
added to all three language files (`messages_en.txt`, `messages_fr.txt`,
`messages_es.txt`) in the same alphabetical position as the other
`oauth.token.*` entries.  Tests `TestExchangeCode_OversizedBody` and
`TestExchangeCode_ExactlyAtLimit` added to `server/oauth/medium_test.go`;
they follow the same pattern as the OAUTH-M4 boundary tests.

