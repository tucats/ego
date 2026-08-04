# SCRIPT-H1 — `readrows` opcode returns at most one row

**Affected files:**

- `server/tables/scripting/rows.go` — `doRows`, `fakeURL` construction

**Description:**
The `readrows` opcode is intended to return all rows that match the specified
filters — an unbounded SELECT. However, `doRows` constructs its internal URL
with a hard-coded `?limit=1` parameter, which was copied from `doSelect` (the
`select` opcode handler) where limiting to one row is the correct behavior:

```go
fakeURL, _ := url.Parse("http://localhost/tables/" + task.Table + "/rows?limit=1")
```

Because `parsing.FormSelectorDeleteQuery` honours the `limit` query parameter
from the URL, this means the `readrows` opcode silently returns at most one row
regardless of how many rows match. Transactions that use `readrows` to
aggregate result sets will receive incomplete data with no error or warning.

The `sql` opcode with a raw `SELECT` statement is unaffected — it bypasses
`fakeURL` entirely and calls the SQL layer directly.

**Test file:** `tools/apitest/tests/4-dsns/dsns-304-tx-readrows.json` — the
test inserts three rows and then calls `readrows`. It expects `count=3`; if
this bug is present the test will fail with `count=1`.

**Recommendation:**
Remove the `?limit=1` fragment from the `fakeURL` in `doRows`, or replace it
with a sufficiently large sentinel value (e.g., `?limit=10000000`). The
`select` opcode (`doSelect`) should retain `?limit=1` since it is a
point-lookup by design. A cleaner solution is to add an explicit `limit` field
to `defs.TXOperation` and pass it through; the `readrows` default would be
unlimited.

**Resolution (May 2026):**
`doRows()` in `server/tables/scripting/rows.go` was modified to remove the
unwanted `?limit=1` parameter on the "readrows" operation. This is distinct
from "select" which intentionally reads a single row to set dictionary variables
to the values of each column. The "readrows" operator wants to read the entire
rowset into memory, as it will be the return data for the @transaction call.

Along the way, fixed an issue where the escape operation "/{{value}}" was poorly
chosen, as it interfered with the injection of dictionary values at the apitest
level into URL paths. The escape was changed to "\{{value}}" to avoid this.
APITEST streams were updated accordingly.

Also, augmented the FullTableName function to be told the provider name, so the
name path could be formed properly according to the provider. Specifically,
if the provider is sqlite3 then we can't and shouldn't use dotted names.

