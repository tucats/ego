# TABLES-M4 — No server-side cap on query result size

**Affected file:** `server/tables/parsing/generators.go:748` — `PagingClauses()`

```go
if limit != 0 {
    result.WriteString(" LIMIT ")
    result.WriteString(strconv.Itoa(limit))
}
// No maximum enforced; if limit == 0 (absent or zero), no LIMIT clause added.
```

**Description:**  
When no `?limit=` query parameter is supplied, or when it is supplied as `0`,
no `LIMIT` clause is appended to the generated SQL. An authenticated caller
can retrieve an entire table — potentially millions of rows — in a single
request. The rows are buffered in the server process before being marshalled
to JSON and written to the response, making this an effective memory-exhaustion
DoS against the server.

**Recommendation:**  
Enforce a server-side maximum: if `limit <= 0 || limit > maxRowLimit`, set
`limit = defaultRowLimit` (e.g. 1000). Expose the maximum as a configurable
setting. Document the behavior so callers can paginate deliberately.

**Resolution:**  
Enforce a server-side maximum row limit in `PagingClauses`; default to 1000
rows when no limit is specified.

