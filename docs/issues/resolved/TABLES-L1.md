# TABLES-L1 — Raw database error messages returned to clients

**Affected files (representative):**

- `server/tables/list.go:51,56`
- `server/tables/transactions.go:225`
- `server/tables/tables.go:320`

```go
msg := fmt.Sprintf("Database list error, %v", err)
util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
```

**Description:**  
Database driver errors — which include table names, column names, constraint
names, SQL syntax fragments, and internal driver details — are forwarded
verbatim to the HTTP response body. An attacker can exploit error responses to
enumerate schema structure, discover column and constraint names without having
read access, and tailor further injection attempts to the precise SQL dialect
in use.

**Recommendation:**  
Log the full error at `ui.DBLogger` and return a generic message
(`"database operation failed"`) to the caller. Reserve detailed messages for
the server log where access is controlled.

**Resolution:**  
Log full database errors server-side and return generic messages in HTTP
error responses from the tables package.

