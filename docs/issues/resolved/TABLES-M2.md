# TABLES-M2 — Row ID concatenated un-parameterized into `UPDATE` WHERE clause

**Affected file:** `server/tables/parsingAbstract.go:77` — `formAbstractUpdateQuery()`

```go
where = "WHERE " + defs.RowIDName + " = '" + idString + "'"
```

**Description:**  
The row ID value taken from the update payload is placed inside a SQL WHERE
clause using string concatenation with single-quote delimiters. While the rest
of the UPDATE statement uses `$N` parameterized placeholders (lines 62–63),
the row-ID filter reverts to direct embedding. A caller who can control the
row ID value (e.g. from the JSON body of a PATCH request) and supplies
`' OR '1'='1` causes the UPDATE to affect every row in the table.

**Recommendation:**  
Append a numbered parameter for the row ID instead of interpolating it:

```go
where = fmt.Sprintf("WHERE %s = $%d", defs.RowIDName, filterCount+1)
// pass idString as the corresponding argument to db.Exec
```

**Resolution:**  
Convert row ID filter in `formAbstractUpdateQuery` to a `$N` numbered
parameter passed to `db.Exec` rather than string-embedded in the WHERE clause.

