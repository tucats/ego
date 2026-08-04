# INDEX-7 — `Array.GetSlice` bounds-checks the wrong backing store and ignores range order

**Affected function:** `(*Array).GetSlice`
**File:** `language/data/arrays.go`
**Risk:** High — panics on a reversed range; rejects every valid slice of a
`[]byte` array
**Status: RESOLVED**

## INDEX-7: Description

```go
if first < 0 || last < 0 || first > len(a.data) || last > len(a.data) {
    return nil, errors.ErrArrayBounds
}

if a.valueType.Kind() == ByteType.kind {
    slice := a.bytes[first:last]
```

Two defects in one guard:

1. **Wrong backing store.** An `Array` keeps its elements in `a.data` *unless*
   the base type is `byte`, in which case they live in `a.bytes` and `a.data` is
   left empty (see the `Array` struct comment and `NewArrayFromBytes`). Both
   bounds were compared against `len(a.data)`, so for a byte array every
   non-empty range was rejected as out of bounds — `GetSlice(0, 5)` on a
   ten-byte array fails because `5 > len(a.data) == 0` — while the byte path
   immediately below sliced `a.bytes` with those same unchecked values.

2. **Range order never checked.** `last < first` was not tested, so
   `GetSlice(5, 2)` on a ten-element array passed the guard and panicked on
   `a.data[5:2]`.

`Set` in the same file already branches on `ByteKind` to pick the right length,
so the correct pattern was established; `GetSlice` simply did not follow it.

## INDEX-7: Fix

The size is taken from whichever store the array's type actually uses, and the
range order is checked:

```go
size := len(a.data)
if a.valueType.Kind() == ByteType.kind {
    size = len(a.bytes)
}

if first < 0 || last < first || first > size || last > size {
    return nil, errors.ErrArrayBounds
}
```
