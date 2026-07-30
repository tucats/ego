# INDEX-9 — `Array.SetSize` pads a byte array using the length of the wrong slice

**Affected function:** `(*Array).SetSize`
**File:** `language/data/arrays.go`
**Risk:** Medium — a grown `[]byte` array ends up the wrong length
**Status: RESOLVED**

## INDEX-9: Description

```go
if a.valueType.Kind() == ByteKind {
    if size < len(a.bytes) {
        a.bytes = a.bytes[:size]
    } else {
        a.bytes = append(a.bytes, make([]byte, size-len(a.data))...)
    }

    return a
}
```

The shrink branch correctly uses `len(a.bytes)`, but the grow branch pads by
`size - len(a.data)`. For a byte array `a.data` is empty, so this appends a full
`size` bytes onto the existing contents and leaves the array at
`len(a.bytes) + size` elements instead of the requested `size`.

Same root cause as INDEX-7 and INDEX-8 — a length taken from the non-active
backing store — but the symptom here is a wrong length rather than a panic or a
dropped write.

## INDEX-9: Fix

```go
a.bytes = append(a.bytes, make([]byte, size-len(a.bytes))...)
```
