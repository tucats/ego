# HTTP-M3 — Redirect server (port 80) has no timeouts

**Affected file:** `commands/server.go:705` — `redirectToHTTPS()`

```go
httpSrv := http.Server{
    Addr:    httpAddr,
    Handler: http.HandlerFunc(...),
}
```

**Description:**  
The plain-HTTP listener created to redirect traffic from port 80 to HTTPS
suffers the same timeout omission as the main server (HTTP-H2). Because it
must remain reachable from the public internet to redirect HTTP clients, it is
the most exposed component, yet it has zero protection against slow-reading
attackers.

**Recommendation:**  
Apply the same `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, and
`IdleTimeout` values to `httpSrv` as recommended in HTTP-H2. For a redirect-
only server, even more aggressive timeouts are appropriate (e.g.,
`ReadHeaderTimeout: 5s`), since legitimate redirect clients will complete their
headers in milliseconds.

**Resolution:**  
Resolved as a side-effect of HTTP-H2: `redirectToHTTPS` now builds its listener
via `makeHTTPServer()`, which applies all four timeout values.

