# SCRIPT-M1 — `_rows_` is always 0 for error conditions after an `insert` opcode

**Affected files:**

- `server/tables/scripting/handler.go` — main dispatch loop, `insertOpcode` case

**Description:**
When processing each opcode the handler maintains two variables that are
exposed to user-defined error condition expressions: `_rows_` (rows affected by
the current operation) and `_all_rows_` (cumulative rows affected so far).

For every opcode except `insert`, the handler captures the return value of the
opcode handler into `count` before evaluating error conditions:

```go
count, httpStatus, operationErr = doSelect(...)   // count updated
...
evalSymbols.SetAlways("_rows_", count)
```

The `insertOpcode` case does not update `count` — it increments `rowsAffected`
directly and ignores the return value of `doInsert`:

```go
case insertOpcode:
    httpStatus, operationErr = doInsert(...)
    rowsAffected++
    // count is never set — still 0 from declaration
```

Because `evalSymbols.SetAlways("_rows_", count)` is evaluated after the
`switch`, `_rows_` is always `0` for `insert` operations regardless of the
actual row count. A user-defined error condition such as `EQ(_rows_, 0)` will
always trigger after an `insert`, even when a row was successfully inserted.

**Recommendation:**
Set `count = 1` immediately before or after `rowsAffected++` in the
`insertOpcode` branch, so that `_rows_` reflects the one row inserted:

```go
case insertOpcode:
    httpStatus, operationErr = doInsert(...)
    count = 1
    rowsAffected++
```

**Resolution (May 2026):**
One change to `server/tables/scripting/handler.go` — `insertOpcode` case:

Added `count = 1` between the `doInsert` call and `rowsAffected++`:

```go
case insertOpcode:
    httpStatus, operationErr = doInsert(session.ID, session.User, db, task, n+1, &dictionary)
    count = 1
    rowsAffected++
```

`doInsert` always inserts exactly one row (its return value is an HTTP status
code, not a row count). Setting `count = 1` unconditionally is correct because
the error-condition evaluation block is guarded by `operationErr == nil`, so
`_rows_` is only read from `count` when the insert actually succeeded.

Two API tests added to `tools/apitest/tests/4-dsns/`:

- `dsns-314-tx-ins-rowcount.json` — inserts a row with an `EQ(_rows_, 0)`
  error condition; before the fix this condition always triggered (returning
  409 Conflict); after the fix `_rows_` is 1 so the condition does not trigger
  and the transaction commits with HTTP 200 and `count=1`.

- `dsns-315-tx-ins-errcond.json` — inserts a row with an `EQ(_rows_, 1)`
  error condition; confirms that a user-defined condition that fires on a
  successful insert (`_rows_ == 1`) correctly rolls back the transaction and
  returns 409 Conflict with the custom error message.

The existing `dsns-314-tx-drop.json` was renumbered to `dsns-316-tx-drop.json`
to preserve alphabetical execution order.

