# TABLES-M1 — SQL injection in `DeleteTable` via un-parameterized `DROP TABLE`

**Affected file:** `server/tables/tables.go:327` — `DeleteTable()`

```go
if dsnName != "" {
    tableName = table                // table = data.String(session.URLParts["table"])
    q = "DROP TABLE " + tableName   // raw concatenation
}
```

**Description:**  
When a DSN name is present in the request, the table-deletion query is
constructed by concatenating the URL path parameter directly into a SQL
string, without quoting or escaping. An authenticated caller who can reach
the delete-table endpoint can supply a table name such as
`users; DROP TABLE admin; --` to execute arbitrary SQL on the DSN's database.
The code path that uses `parsing.QueryParameters` (lines 316–318) is bypassed
entirely in the DSN case.

**Recommendation:**  
Use double-quote escaping consistent with the non-DSN path:
`q = "DROP TABLE \"" + tableName + "\""`. Better still, route through
`parsing.QueryParameters` regardless of whether a DSN is present.

**Resolution:**  
Replace `"DROP TABLE " + tableName` in the DSN branch of `DeleteTable` with a
quoted identifier; route through `parsing.QueryParameters` for consistency.

