# OAUTH-M2 — No timeout on outbound IdP HTTP calls

**Affected files:**

- `server/oauth/discovery.go:79` — `discoverEndpoints()`: `http.Get(discoveryURL)`
- `server/oauth/jwks.go:89` — `refreshJWKS()`: `http.Get(jwksURL)`
- `server/oauth/flow_authcode.go:143` — `ExchangeCode()`: `http.DefaultClient.Do(req)`
- `app-cli/app/logon_oauth.go:201` — `fetchOIDCDiscovery()`: `http.Get(discoveryURL)`
- `app-cli/app/logon_oauth.go:374` — `postTokenRequest()`: `http.DefaultClient.Do(req)`

**Description:**
All outbound HTTP calls to the identity provider (discovery, JWKS fetch, and token
exchange) use either `http.Get` or `http.DefaultClient.Do`, neither of which sets a
deadline. If the IdP is slow or unresponsive, each goroutine handling an inbound
request blocks indefinitely on the outbound call. A network partition, an
overloaded IdP, or a deliberate slowdown by a malicious upstream server can
exhaust all available server goroutines — effectively causing a DoS of the Ego
server without any involvement of the attacker's client.

This is analogous to the Slowloris vulnerability described in HTTP-H2, but for
outbound connections rather than inbound ones.

**Recommendation:**
Replace `http.DefaultClient` with a client that carries a context deadline derived
from the inbound request, or use a package-level client with a bounded timeout:

```go
var idpClient = &http.Client{Timeout: 10 * time.Second}
```

Apply this to `discoverEndpoints`, `refreshJWKS`, `ExchangeCode`, and the CLI's
`postTokenRequest`. The discovery and JWKS fetches happen at startup and on cache
miss; 10–30 seconds is a reasonable wall-clock limit. The token exchange happens
per-request; 10 seconds matches common IdP SLAs.

**Resolution (May 2026):**
A new file `server/oauth/client.go` defines a package-level `idpClient =
&http.Client{Timeout: 10 * time.Second}`.  All three server-side outbound call
sites — `discoverEndpoints` (`discovery.go`), `refreshJWKS` (`jwks.go`), and
`ExchangeCode` (`flow_authcode.go`) — now use `idpClient.Get(...)` /
`idpClient.Do(...)` instead of `http.Get` / `http.DefaultClient.Do`.  The CLI
gains a parallel `oauthHTTPClient = &http.Client{Timeout: 30 * time.Second}` in
`app-cli/app/logon_oauth.go` (30 s is more generous for a user-facing flow).
Both `fetchOIDCDiscovery` and `postTokenRequest` in the CLI now use
`oauthHTTPClient`.  Tests in `server/oauth/medium_test.go` verify that
`idpClient.Timeout` is non-zero and that the client is actually used for
discovery requests.

