# SCRIPT-M2 — `drop` opcode does not flush the schema cache

**Affected files:**

- `server/tables/scripting/handler.go` — `needCacheFlush` logic
- `server/tables/scripting/drop.go` — `doDrop`

**Description:**
The server maintains an in-memory schema cache to avoid repeated database
introspection for table column lists and types. After a DDL change the cache
must be flushed so that subsequent operations see the updated schema.

The handler sets `needCacheFlush = true` only when `sqlOpcode` returns a
flush signal — specifically for `ALTER TABLE` statements executed via the `sql`
opcode. The `drop` opcode (`dropOpCode` / `doDrop`) does not set
`needCacheFlush`, meaning the schema cache retains an entry for a table that
no longer exists.

In a test sequence that drops and recreates a table (or in any transaction that
drops a table and then the same connection subsequently references that table),
a stale cache entry may produce `"column 'x' does not exist"` errors or
incorrect column type metadata.

**Recommendation:**
Either have `doDrop` return a flush-needed flag (parallel to the `doSQL`
signature) and propagate it to `needCacheFlush`, or unconditionally set
`needCacheFlush = true` in the `dropOpCode` case of the handler dispatch loop.

**Resolution (May 2026):**
Two changes, fixing both the `drop` opcode and the `sql` opcode `DROP TABLE`
path simultaneously:

**`server/tables/scripting/handler.go` — `dropOpCode` case:**

```go
case dropOpCode:
    httpStatus, operationErr = doDrop(session.ID, session.User, db, task, n+1, &dictionary)
    if operationErr == nil {
        needCacheFlush = true
    }
```

The guard `operationErr == nil` ensures that a failed drop (e.g. table does not
exist → 404) does not spuriously flush the cache before the transaction is rolled
back.

**`server/tables/scripting/sql.go` — `doSQL`, `cacheFlush` detection:**

```go
// Before: ALTER TABLE only
if len(tokens) > 2 && tokens[0] == "alter" && tokens[1] == "table" {
    cacheFlush = true
}
// After: ALTER TABLE and DROP TABLE
if len(tokens) > 2 && tokens[1] == "table" && (tokens[0] == "alter" || tokens[0] == "drop") {
    cacheFlush = true
}
```

`CREATE TABLE` is intentionally excluded: a brand-new table has no stale cache
entry to evict.

**API tests added** in `tools/apitest/tests/4-dsns/`:

*Subgroup A — `drop` opcode (table `sca{{SQLUUID}}`, tests 317–321):*

- `dsns-317-tx-sca-create.json` — creates `sca{{SQLUUID}}` with two columns (id, name) and inserts one row.
- `dsns-318-tx-sca-warm.json` — `GET .../sca{{SQLUUID}}/rows` warms the `SchemaCache` with the 2-column layout.
- `dsns-319-tx-sca-drop-recreate.json` — `@transaction` with `drop sca{{SQLUUID}}` + `sql CREATE TABLE sca{{SQLUUID}} (..., extra TEXT)` + `insert {id:2, name:"after", extra:"bonus"}`.
- `dsns-320-tx-sca-verify.json` — `GET .../sca{{SQLUUID}}/rows?columns=extra` must return **200** with `rows.0.extra == "bonus"`. Without the fix, the stale cache does not know about `extra` and `ReadRows` returns **400 "invalid column name: extra"`.
- `dsns-321-tx-sca-cleanup.json` — drops `sca{{SQLUUID}}`.

*Subgroup B — `sql` opcode `DROP TABLE` (table `scb{{SQLUUID}}`, tests 322–326):*

- `dsns-322-tx-scb-create.json` — same setup with `scb{{SQLUUID}}`.
- `dsns-323-tx-scb-warm.json` — warms the cache for `scb{{SQLUUID}}`.
- `dsns-324-tx-scb-sqldrop-recreate.json` — `@transaction` with `sql "DROP TABLE scb{{SQLUUID}}"` + `sql CREATE TABLE` (with `extra`) + `insert`.
- `dsns-325-tx-scb-verify.json` — `GET .../scb{{SQLUUID}}/rows?columns=extra` must return **200**; same failure mode without the fix.
- `dsns-326-tx-scb-cleanup.json` — drops `scb{{SQLUUID}}`.

The verify tests (`dsns-320`, `dsns-325`) are the regression anchors: they pass
only when the schema cache has been flushed and `ReadRows` re-queries the
database to obtain the updated column list for the recreated table.

