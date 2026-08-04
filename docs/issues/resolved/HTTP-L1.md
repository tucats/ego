# HTTP-L1 — User-supplied URL path reflected verbatim in error messages

**Affected file:** `server/server/serve.go:79`

```go
msg = "endpoint " + r.URL.Path + " not found"
```

**Description:**  
When a route is not found, the raw URL path from the request is concatenated
directly into the error message string that is returned to the client (and
written to the server log). This is not a direct injection risk for JSON-
consuming API clients, but it has two minor consequences:

1. **Information disclosure** — The exact URL string the attacker sent is
   reflected back, confirming what path patterns do and do not exist on the
   server. This makes reconnaissance easier.
2. **Log injection** — If the URL path contains newline characters or log-
   format control sequences, they appear verbatim in the structured SERVER log
   entry, potentially corrupting log output or confusing log parsers.

**Recommendation:**  
Return a generic message to the client (`"not found"`) and include the raw path
only in the server-side log:

```go
util.ErrorResponse(w, sessionID, "not found", status)
ui.Log(ui.ServerLogger, "server.route.error", ui.A{
    ...
    "path": r.URL.Path,  // path stays in the log, not in the response
})
```

**Resolution:**  
Generic `"not found"` / `"forbidden"` returned to client; raw URL path kept
only in the `server.route.error` log entry.

