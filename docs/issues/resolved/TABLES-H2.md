# TABLES-H2 — Inverted authorization filter in `ListTables` leaks table names

**Affected file:** `server/tables/list.go:136` — `getTableNames()`

```go
if !db.Session.Admin && Authorized(db.Session, db.Session.User, name, defs.TableReadPermission) {
    continue  // skips tables the user IS authorized to read
}
```

**Description:**  
The `!` before `Authorized` is missing. `Authorized` returns `true` when the
user has the requested permission. The current code therefore skips (hides)
every table the user *is* permitted to read, and exposes every table they are
*not* permitted to read.

For non-secured DSNs (the common case) `Authorized` always returns `true`, so
the `continue` fires for every table and non-admin users see an empty list —
a functional denial of service. For secured DSNs the effect is reversed access
control: tables a non-admin user may read are hidden from them, while tables
they have no business seeing are visible.

**Recommendation:**  
Add the negation: `if !db.Session.Admin && !Authorized(...)`.

**Resolution:**  
Add missing `!` to `Authorized` call in `getTableNames` so tables the user
cannot read are filtered out, not those they can.

