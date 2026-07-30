# INDEX-8 — `Array.SetAlways` bounds-checks `a.data` but writes `a.bytes`

**Affected function:** `(*Array).SetAlways`
**File:** `language/data/arrays.go`
**Risk:** High — every write to a `[]byte` array is silently discarded
**Status: RESOLVED**

## INDEX-8: Description

```go
if index < 0 || index >= len(a.data) {
    return a
}

if a.valueType.Kind() == ByteKind {
    a.bytes[index], _ = Byte(value)
} else {
    a.data[index] = value
}
```

The guard and the write disagree about which slice holds the elements. For a
`[]byte` array the data is in `a.bytes` and `a.data` is empty, so
`index >= len(a.data)` is true for *every* index, including 0 — the function
returns early and writes nothing. `SetAlways` is therefore a complete no-op on
byte arrays, and because it returns the receiver rather than an error, the caller
has no way to notice.

This is the mirror image of INDEX-7: there the mismatch rejected valid reads,
here it discards valid writes. `Set`, in the same file, gets this right by
branching on the kind before choosing the length to compare against.

## INDEX-8: Fix

The bounds check now uses the same backing store as the write:

```go
if a.valueType.Kind() == ByteKind {
    if index < 0 || index >= len(a.bytes) {
        return a
    }
} else if index < 0 || index >= len(a.data) {
    return a
}
```
