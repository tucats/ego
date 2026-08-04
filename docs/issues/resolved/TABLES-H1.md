# TABLES-H1 — SQL injection via raw username in table-list query

**Affected file:** `server/tables/list.go:69` — `listTables()`

```go
schema := session.User
q := strings.ReplaceAll(tablesListQuery, "{{schema}}", schema)
// tablesListQuery = `... WHERE table_schema = '{{schema}}' ORDER BY ...`
```

**Description:**  
The authenticated user's name is substituted directly into a SQL string using
`strings.ReplaceAll`, bypassing the `parsing.QueryParameters` / `SQLEscape`
pipeline that all other query parameters go through. The value lands inside a
single-quoted SQL literal, but `SQLEscape` is never called. A username whose
middle characters include a single quote (e.g. `O'Reilly`) breaks the literal
and allows injection:

```sql
WHERE table_schema = 'O'Reilly' ORDER BY table_name
```

A more deliberately crafted name (`x' UNION SELECT username,passwd FROM
pg_shadow--`) could exfiltrate sensitive data from the database.

**Recommendation:**  
Route the schema substitution through `parsing.QueryParameters`, which calls
`SQLEscape` on every value. Alternatively, use a parameterized query where the
driver handles quoting: `db.Query("... WHERE table_schema = $1 ...", schema)`.

**Resolution:**  
Route schema substitution in `listTables` through `parsing.QueryParameters`
(which calls `SQLEscape`) instead of bare `strings.ReplaceAll`.

