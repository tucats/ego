# JSON-H1 — Unmarshal nested struct stores raw Go map instead of Ego struct

**Affected files:**

- `runtime/json/unmarshal.go` — `remapDecodedValue`, `case *data.Struct`

**Description:**
When `json.Unmarshal` writes to an Ego struct and the JSON contains a nested
object, the nested object is stored in the struct field as a raw
`map[string]interface{}` (a Go native map) rather than as the corresponding
Ego struct type that the field was initialized with.

```go
inner := {x: 0, y: 0}
outer := {label: "", pt: inner}
err := json.Unmarshal([]byte(`{"label":"origin","pt":{"x":1,"y":2}}`), &outer)
// outer.pt is now map[string]interface{}, not {x, y}
// reflect.Type(outer.pt) == interface{}   ← not the struct type
```

The root cause is that `remapDecodedValue` iterates over the decoded JSON map
and calls `target.Set(k, v)` with the raw Go value `v` for each key. It does
not check whether the field's declared type is a struct and does not recursively
convert the nested map to that struct type.

Go's `encoding/json` handles this by using reflection on typed struct fields.
Ego's implementation would need to inspect the type of each struct field (via
`target.FieldTypes()` or similar) and call `data.NewStructOfTypeFromMap` when
the field type is a struct kind.

**Test file:** `tests/json/unmarshal.ego` —
`"json: Unmarshal - nested struct stores raw map"` documents this behavior.

**Recommendation:**
In `remapDecodedValue` under `case *data.Struct`, for each key-value pair look
up the declared field type. If the field type is `StructKind` and the decoded
value is `map[string]any`, call `data.NewStructOfTypeFromMap(fieldType, m)` to
create a proper Ego struct before calling `target.Set(k, v)`. This is the same
pattern already applied to struct elements in the `case *data.Array` path.

**Resolution (May 2026):**
One change to `runtime/json/unmarshal.go` — `remapDecodedValue`, `case *data.Struct`:

Inside the `for k, v := range m` loop, before the `target.Set(k, v)` call, a
type-check was added:

```go
if mm, ok := v.(map[string]any); ok {
    if fieldType, fErr := target.Type().Field(k); fErr == nil && fieldType.Kind() == data.StructKind {
        v = data.NewStructOfTypeFromMap(fieldType, mm)
    }
}
```

`target.Type().Field(k)` (`data/types.go:1299`) returns the declared `*data.Type`
for field `k`. When its kind is `StructKind` and the decoded JSON value is a
`map[string]any`, `data.NewStructOfTypeFromMap` converts the raw map into a
properly-typed Ego struct before it is stored. Fields whose declared type is not
`StructKind` (scalars, arrays, maps) fall through unchanged. Unknown field names
produce a non-nil `fErr` and also fall through, letting the existing
`target.Set(k, v)` error path handle them (covered separately by JSON-M3).

This is the same pattern already used in the `case *data.Array` branch (lines
123–126 before this change).

Test in `tests/json/unmarshal.ego` renamed from
`"json: Unmarshal - nested struct stores raw map"` to
`"json: Unmarshal - nested struct field is correct type"` and updated to assert
that `outer.pt.x == 1` and `outer.pt.y == 2` after unmarshal rather than
asserting the buggy `interface{}` type.

