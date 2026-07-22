# HTTP-L2 — Request body read after permission check already set a failure status

**Affected file:** `server/server/serve.go:302–318`

**Description:**  
When the `mustBeAdmin` check fails (the user is authenticated but lacks admin
privileges), the code sets `status = http.StatusForbidden` but does **not**
`return`. Execution falls through to the unconditional `io.ReadAll(r.Body)` on
line 313, which reads the full request body into memory before the handler
(correctly) declines to run. The same pattern applies to the
`mustAuthenticate + canAuthenticate` branch that sets 401. In both cases a
non-privileged caller can force the server to allocate memory for whatever body
they attach to a privileged endpoint. While the individual impact is low, it
directly compounds HTTP-H1 (no body size limit): a stream of 403-destined
requests with large bodies can exhaust memory without ever triggering the
handler path.

**Recommendation:**  
Add an early `return` after each auth/permission failure that sets a non-OK
status, so the body is never read for requests that are going to be rejected:

```go
} else if route.mustBeAdmin && !session.Admin {
    ...
    util.ErrorResponse(w, session.ID, "not authorized", http.StatusForbidden)
    return  // ← add this
}
```

**Resolution:**  
`mustAuthenticate` and `mustBeAdmin` failure branches in `ServeHTTP` now
`return` immediately after sending the error response; request body is never
read for rejected requests.

