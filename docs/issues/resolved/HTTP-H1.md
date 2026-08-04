# HTTP-H1 — No request body size limit

**Affected file:** `server/server/serve.go:313`

```go
session.Body, _ = io.ReadAll(r.Body)
```

**Description:**  
Every non-lightweight request has its body read into memory with a bare
`io.ReadAll`. There is no call to `http.MaxBytesReader` and no size check
before the read. An attacker — authenticated or not — can send a POST or PUT
request with a body of arbitrary size (limited only by their bandwidth and the
server's available RAM). Because the read happens unconditionally before the
handler is invoked, even endpoints that ignore the body will fully consume it.
A single large request can cause the Go garbage collector to thrash; a flood of
them can exhaust virtual memory and crash the process.

**Recommendation:**  
Wrap the request body with `http.MaxBytesReader` before calling `io.ReadAll`.
Choose a generous-but-bounded limit appropriate to the largest legitimate
payload (e.g., 32 MiB for the Ego server's use cases, configurable via a
setting):

```go
const maxBodyBytes = 32 << 20  // 32 MiB

r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
session.Body, _ = io.ReadAll(r.Body)
```

`http.MaxBytesReader` returns a `*http.MaxBytesError` when the limit is
exceeded, which `io.ReadAll` surfaces as a non-nil error. Check the error and
return 413 Request Entity Too Large.

**Resolution:**  
`http.MaxBytesReader` wraps `r.Body` before `io.ReadAll`; returns 413 on
oversize body; limit defaults to 32 MiB, configurable via
`ego.server.max.body.size`.

