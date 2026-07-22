# HTTP-H2 — No timeouts on the HTTP server — Slowloris vulnerability

**Affected files:**

- `commands/server.go:250` — plain HTTP path: `http.ListenAndServe(addr, router)`
- `commands/server.go:506` — TLS path: `http.ListenAndServeTLS(addr, certFile, keyFile, router)`

**Description:**  
Both server start paths use the convenience functions `http.ListenAndServe` and
`http.ListenAndServeTLS`, which create an `http.Server` with all timeout fields
left at their zero value — meaning no timeout at all. This makes the server
vulnerable to the Slowloris attack: an attacker opens many connections and
sends HTTP request headers one byte at a time, never completing them. Each
such connection holds a Go goroutine and a file descriptor open indefinitely.
With enough connections (default Linux limit is typically 1024 open file
descriptors per process), the server stops accepting new legitimate requests.
No authentication is required — the attack happens before the request headers
are complete.

**Recommendation:**  
Replace the convenience functions with an explicit `http.Server` that sets
all four timeout fields. Recommended starting values:

```go
srv := &http.Server{
    Addr:              addr,
    Handler:           router,
    ReadHeaderTimeout: 10 * time.Second,  // time to receive all headers
    ReadTimeout:       60 * time.Second,  // time to receive the full request
    WriteTimeout:      120 * time.Second, // time to send the full response
    IdleTimeout:       120 * time.Second, // keep-alive idle before closing
}
err = srv.ListenAndServeTLS(certFile, keyFile)
```

`ReadHeaderTimeout` is the most critical: it directly stops the Slowloris
pattern by closing connections that do not finish sending headers within the
window. The values above are reasonable defaults; consider making them
configurable via server settings for environments with large response bodies
(e.g., log retrieval).

**Resolution:**  
`makeHTTPServer()` helper constructs `http.Server` with `ReadHeaderTimeout`
(10 s), `ReadTimeout` (30 s), `WriteTimeout` (120 s), and `IdleTimeout` (120 s);
all three listeners (plain HTTP, TLS, and HTTP→HTTPS redirect) use it; all four
values are configurable via `ego.server.{read.header|read|write|idle}.timeout`.

