# TABLES-M3 — Wrong permission constant in `DeleteTable` allows any authenticated user to delete tables

**Affected file:** `server/tables/tables.go:312` — `DeleteTable()`

```go
if !isAdmin && dsnName == "" && !Authorized(session, user, tableName, defs.AdminAgent) {
    return util.ErrorResponse(..., "User does not have read permission", ...)
}
```

**Description:**  
Two bugs combine here. First, `defs.AdminAgent` has the value `"admin"` — a
string constant identifying an agent type, not a table-permission name. The
`Authorized` switch statement has no case for `"admin"`, so the per-operation
loop runs without ever setting `auth = false`, and `Authorized` returns `true`
for any authenticated user. The guard `!Authorized(...)` is therefore always
`false`, meaning no non-admin user is ever blocked from deleting a table they
do not own. Second, the error message says "read permission" when the intent
is an admin/delete permission check.

**Recommendation:**  
Replace `defs.AdminAgent` with `defs.TableAdminPermission` (= `"ego.table.admin"`)
and update the error message accordingly.

**Resolution:**  
Replace `defs.AdminAgent` with `defs.TableAdminPermission` in the `DeleteTable`
authorization check; correct the error message from "read permission" to
"admin permission".

