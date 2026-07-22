# OAUTH-M4 — OIDC discovery and JWKS responses read without a size limit

**Affected files:**

- `server/oauth/discovery.go:86` — `discoverEndpoints()`: `io.ReadAll(resp.Body)`
- `server/oauth/jwks.go:99` — `refreshJWKS()`: `io.ReadAll(resp.Body)`

**Description:**
Both functions read the IdP's HTTP response body into memory with a bare
`io.ReadAll`, applying no size limit before the allocation. A malicious or
compromised IdP, or a network attacker who can intercept the outbound TLS
connection (e.g., via a CA compromise), can return an arbitrarily large response
body. The full response is allocated into a single byte slice, so even a
moderately large payload (tens of megabytes) causes a visible memory spike;
gigabyte payloads can exhaust heap and crash the server.

Discovery documents and JWKS responses are inherently small — a few kilobytes at
most for any realistic deployment. A generous limit of 1 MiB is far more than
any legitimate document requires.

**Recommendation:**
Wrap the response body with `http.MaxBytesReader` (or `io.LimitReader`) before
reading:

```go
body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB cap
```

A response that exceeds the limit should be treated as an error and logged so the
operator can investigate.

**Resolution (May 2026):**
Both `discoverEndpoints` (`discovery.go`) and `refreshJWKS` (`jwks.go`) now wrap
`resp.Body` in `&io.LimitedReader{R: resp.Body, N: maxDiscoveryBytes + 1}` (where
`maxDiscoveryBytes = 1 << 20`, 1 MiB) before calling `io.ReadAll`.  Using N+1
rather than N avoids an off-by-one: when `lr.N` reaches zero after reading,
it proves the body is strictly larger than the limit rather than exactly equal to
it.  A zero `lr.N` after `io.ReadAll` returns causes the function to return a
descriptive error containing "exceeds" so the operator knows why the document was
rejected.  Tests in `server/oauth/medium_test.go` cover: a body one byte over the
limit is rejected, a body exactly at the limit passes the size check (then fails
JSON parsing, confirming no off-by-one regression).

