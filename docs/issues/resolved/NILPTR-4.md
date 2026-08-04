# NILPTR-4 — `Database.Close` dereferences a handle that `Open` may never have set

**Affected functions:** `(*Database).Close`, `(*Database).CloseTX`
**File:** `server/tables/database/open.go`
**Risk:** Low — not reachable through any current caller, but the failure shape
is one that a routine `defer` placement mistake would expose
**Status: RESOLVED**

## NILPTR-4: Description

`Open` can return a non-nil `*Database` whose `Handle` field is still nil. There
are two such paths:

```go
default:
    // The scheme from the DSN connection string does not correspond to any
    // provider known to this server.
    return db, errors.ErrUnsupportedDatabase.Context(scheme)
```

and the case where `egostrings.FindScheme(conStr)` returns an error, which skips
the entire `if err == nil` block that would have called `sql.Open`.

`Close` then did:

```go
func (d *Database) Close() error {
    if d.Transaction != nil {
        ...
        return nil
    }

    return d.Handle.Close()
}
```

No check on `d`, and none on `d.Handle`.

All three current callers happen to test the error before deferring `Close`, so
this is not reachable today. It is documented and fixed anyway because of how
easy the mistake is to make: `defer db.Close()` written one line above the error
check is an extremely common Go idiom slip, and it would turn a clean
"unsupported database" error into a panic in a request handler.

Worth knowing for anyone new to Go: calling a method on a nil pointer is
perfectly legal, and the method runs — it only panics if the method actually
dereferences the receiver. That makes a nil-receiver check a complete and very
cheap fix, rather than something the caller has to remember.

## NILPTR-4: Fix

Both methods now check the receiver and the handle:

```go
func (d *Database) Close() error {
    if d == nil {
        return nil
    }

    if d.Transaction != nil {
        ...
        return nil
    }

    // A database that never finished opening has nothing to close.
    if d.Handle == nil {
        return nil
    }

    return d.Handle.Close()
}
```

`CloseTX` received the same two guards.
