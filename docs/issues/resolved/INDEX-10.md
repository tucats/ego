# INDEX-10 — `explode` indexes `r.Columns` with a reflected struct's field count

**Affected function:** `(*ResHandle).explode`
**File:** `resources/describe.go`
**Risk:** Medium — panics when the object passed in does not match the handle
**Status: RESOLVED**

## INDEX-10: Description

```go
count := value.NumField()
result = make([]any, count)

for i := 0; i < count; i++ {
    field := value.Field(i)
    ...
    if r.Columns[i].IsRawJSON {
```

The loop bound comes from the reflected struct passed in by the caller, but the
loop counter also indexes `r.Columns`, a separate slice built by `describe()`
from the resource's table definition. Nothing ties the two lengths together —
`explode` accepts `object any` — so a struct with more fields than the handle has
columns walks `r.Columns` past its end and panics.

## INDEX-10: Fix

The field count is clamped to the number of columns actually described, and the
mismatch is logged:

```go
count := value.NumField()
if count > len(r.Columns) {
    ui.Log(ui.ResourceLogger, "resource.explode.fields", ui.A{
        "fields":  count,
        "columns": len(r.Columns)})

    count = len(r.Columns)
}
```

A mismatched struct now yields the columns the handle knows about instead of
crashing. A new `resource.explode.fields` message was added to all four language
files.
