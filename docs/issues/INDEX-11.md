# INDEX-11 — `UpdateOne` uses a column index to index the exploded value list

**Affected function:** `(*ResHandle).UpdateOne`
**File:** `resources/update.go`
**Risk:** Medium — panics on a nil or short exploded object
**Status: RESOLVED**

## INDEX-11: Description

```go
keyIndex := r.PrimaryKeyIndex()
if keyIndex < 0 {
    return errors.ErrNotFound
}

items := r.explode(v)

return r.Update(v, r.Equals(r.Columns[keyIndex].SQLName, items[keyIndex]))
```

`keyIndex` is a position within `r.Columns`, established by `PrimaryKeyIndex()`.
It is then used unchanged to index `items`, the result of `explode(v)` — a
different slice whose length depends on the object the caller supplied.

`explode` returns `nil` outright when `v` is not a struct, which makes
`items[keyIndex]` an index into a nil slice. Even for a struct, the two lengths
agree only when `v` is exactly the type this handle describes.

## INDEX-11: Fix

The primary key value is read only when the exploded object actually reaches
that position:

```go
items := r.explode(v)
if keyIndex >= len(items) {
    return errors.ErrNotFound.Clone().Context(r.Columns[keyIndex].SQLName)
}
```
