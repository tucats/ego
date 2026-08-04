# TABLES-L2 — SQLite `PRAGMA` statements use unquoted table and index names

**Affected file:** `server/tables/describe.go:244,276,307`

```go
q := fmt.Sprintf("PRAGMA index_list(%s)", tableName)
q := fmt.Sprintf("PRAGMA index_info(%s)", index)
q  = fmt.Sprintf("PRAGMA table_info(%s)", tableName)
```

**Description:**  
SQLite PRAGMA arguments are not parameterizable via `database/sql`, but names
with spaces or special characters still need to be quoted. The table names
passed here come from the server's own schema metadata (a prior `sqlite_schema`
query), so the practical risk is low — an attacker would need to have already
created a table with a malicious name. Nevertheless, the pattern violates the
principle of always quoting identifiers and would allow a name such as
`my table` (with a space) to silently truncate the PRAGMA to `PRAGMA
index_list(my)`.

**Recommendation:**  
Wrap each name in backticks or double-quotes:

```go
q := fmt.Sprintf("PRAGMA index_list(\"%s\")", tableName)
```

**Resolution:**  
Wrap table and index names in double-quotes in all three SQLite `PRAGMA`
format strings in `describe.go`.

