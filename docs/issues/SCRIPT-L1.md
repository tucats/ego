# SCRIPT-L1 — Empty transaction body returns plain text, not JSON

**Affected files:**

- `server/tables/scripting/handler.go` — empty-task early return (lines 51–59)

**Description:**
When a `POST /@transaction` request is received with an empty body (zero
operations), the handler writes a plain text message and returns HTTP 200:

```go
text := i18n.T("msg.table.tx.empty")
w.WriteHeader(http.StatusOK)
_, _ = w.Write([]byte(text))
```

No `Content-Type` response header is set, so the response defaults to
`text/plain; charset=utf-8`. Every other `@transaction` response carries a
structured `application/vnd.ego.*+json` content type. A client that inspects
the `Content-Type` header to decide how to parse the response will need a
special case for the empty-body path.

**Recommendation:**
Replace the plain text response with a `rowcount+json` response carrying
`count: 0`, consistent with how a non-empty transaction with zero affected rows
is encoded. This makes all `@transaction` success responses uniform and
removes the special-case parsing requirement from clients.

**Resolution:**
The empty-body path now returns a structured JSON response consistent with all
other `@transaction` responses, rather than a plain-text body with no
`Content-Type` header. The response returns HTTP 200 with the message field set
to `"No transactions in task"`, matching the format used by non-empty
transaction responses.

