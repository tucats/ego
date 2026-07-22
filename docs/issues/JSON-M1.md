# JSON-M1 — Marshal of []byte produces "null" instead of base64 string

**Affected files:**

- `data/sanitize.go` — `Sanitize` function, `*data.Array` of `ByteKind`

**Description:**
Go's `encoding/json` encodes a `[]byte` value as a base64-encoded JSON string.
In Ego, `json.Marshal([]byte{65, 66, 67})` produces `"null"` instead of
`"QUJD"`.

The cause is that `data.Sanitize` — which converts Ego runtime values to Go
native types before passing them to `json.Marshal` — does not convert a
`*data.Array` whose element type is `ByteKind` into a native Go `[]byte` slice.
The sanitized value ends up as `nil`, which `json.Marshal` encodes as `null`.

```go
byt := []byte{65, 66, 67}
b, err := json.Marshal(byt)
// Expected:  b == []byte(`"QUJD"`)
// Actual:    b == []byte("null")
```

This also affects `json.WriteFile` when given a byte array value to write.

**Test file:** `tests/json/marshal.ego` —
`"json: Marshal - []byte produces null"` documents the current (incorrect)
behavior.

**Recommendation:**
In `data.Sanitize`, add a case for `*data.Array` values whose `Type().Kind()`
is `data.ByteKind`: call `array.GetBytes()` and return the resulting `[]byte`
slice. This will allow `json.Marshal` to receive a native `[]byte` and produce
the standard base64 encoding.

**Resolution (May 2026):**
One change to `data/sanitize.go` — `Sanitize`, `case *Array`:

Before returning `v.data`, a `ByteKind` check was added:

```go
if v.Type().Kind() == ByteKind {
    return v.GetBytes()
}
```

`GetBytes()` returns the internal `[]byte` backing the byte array. Passing that
native slice to `json.Marshal` triggers Go's standard base64 encoding. For all
other array element types the existing `v.data` path is unchanged.

Test in `tests/json/marshal.ego` renamed from
`"json: Marshal - []byte produces null"` to
`"json: Marshal - []byte produces base64 string"` and updated to assert
`string(b)` equals the JSON string `"QUJD"` (the base64 encoding of `[65,66,67]` = `"ABC"`).

