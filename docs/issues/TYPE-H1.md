# TYPE-H1 — json.Unmarshal cannot reconstruct deeply nested or array-element types

**Affected files:**

- `runtime/json/unmarshal.go` — `remapDecodedValue`

**Description:**
Go's `encoding/json` decodes all JSON into a small set of native Go types:
`map[string]any`, `[]any`, `float64`, `string`, `bool`, and `nil`. Ego's
`remapDecodedValue` then attempts to convert that generic tree into typed Ego
values, guided by the destination pointer. The current implementation is a flat
`switch` on the destination's runtime type (`*data.Struct`, `*data.Map`,
`*data.Array`, scalar) with case-specific conversion logic at each level. This
approach has two related gaps:

**Gap 1 — Array elements are not converted when the element type is `interface{}`.**

The standard Ego idiom for an array of structs is `[]any{pt, pt}`, which gives the
array an element type of `InterfaceKind`. The existing `case *data.Array` code only
converts `map[string]any` elements to Ego structs when `target.Type().Kind() ==
data.StructKind`. For `[]any` the check is always false, so JSON array elements that
represent objects are stored as raw `map[string]any` rather than the struct values
the array already contained before the unmarshal call.

```go
pt := {x: 0, y: 0}
arr := []any{pt, pt}
err := json.Unmarshal([]byte(`[{"x":1,"y":2},{"x":3,"y":4}]`), &arr)
// arr[0] is map[string]interface{}, not {x:1, y:2}
```

The existing elements of `arr` before the call carry the struct type information
(they are `*data.Struct` values), but `SetSize` is called before they are examined,
and the conversion loop has no access to that type hint.

**Gap 2 — Nesting beyond one level is not handled.**

The fix for JSON-H1 converts a `map[string]any` value into a struct when the
containing struct's field type is `StructKind`. But if that nested struct itself has
a field that is an array of structs, or if a map value is a struct, those inner
conversions are not applied. Only one level of nesting is handled in any given
`case` branch.

**Root cause:**
The flat `switch`-based approach in `remapDecodedValue` is inherently limited to
one level of conversion per call. Arbitrarily deep nesting (arrays of structs
containing arrays of maps containing structs, etc.) requires a recursive traversal
of the decoded value tree guided by the destination object's type information at
every level.

**Recommendation:**
Redesign `remapDecodedValue` as a recursive function. The strategy:

1. Let `encoding/json` decode normally into a bare `any` (current behavior —
   no change here).
2. Replace the flat `switch` with a recursive `convertToEgoType(decoded any,
   targetType *data.Type) any` that walks both the decoded value tree and the
   declared type tree simultaneously:
   - If the target type is `StructKind` and the decoded value is `map[string]any`,
     create a new `*data.Struct` of that type and recursively convert each field
     value by looking up the field's declared type.
   - If the target type is `ArrayKind` and the decoded value is `[]any`, create a
     new `*data.Array` of the declared element type and recursively convert each
     element.
   - If the target type is `MapKind` and the decoded value is `map[string]any`,
     create a new `*data.Map` and recursively convert each value using the map's
     declared value type.
   - For scalar types, apply `data.Coerce` as today.
   - If the target type is `InterfaceKind`, apply a best-effort conversion: a
     `map[string]any` becomes an Ego `*data.Map`, a `[]any` becomes a `*data.Array`
     of `InterfaceType`, scalars are kept as-is. This preserves the current
     no-model behavior.
3. The entry point (`remapDecodedValue`) seeds the recursion with the declared type
   of the destination pointer's current value.

This approach handles arbitrary nesting depth, eliminates the per-type special
cases, and makes it straightforward to add future target types (e.g., typed
channels, user-defined types).

**Resolution (May 2026):**
`remapDecodedValue` was refactored and a new `reconstructValue(decoded any, model any) (any, error)`
helper was added in `runtime/json/unmarshal.go`.

**Design:** Instead of a `*data.Type`-based recursive target type, the helper takes the
*existing Ego value* at each position as the `model` and uses its concrete Go type (via a
`switch` on the model) to drive conversion. This sidesteps the need for an unexported
`ValueType()` accessor on `*data.Type` and works naturally with Ego's runtime representation:

- `model = *data.Struct` → creates a new `*data.Struct` of the same type, recurses into each field using `m.Get(fieldName)` as the per-field model.
- `model = *data.Array` → creates a new `*data.Array` with the same element type (`m.Type()`), uses the original element at each index (or the first element as a fallback for new slots) as the per-element model.
- `model = *data.Map` → creates a new `*data.Map` with the same key and value types; uses `InstanceOfType(m.ElementType())` as the per-value model (or `nil` when element type is `interface{}` to avoid spurious coercion).
- scalar / nil model → attempt `data.Coerce` to the model's type; fall back to best-effort (`map[string]any` → Ego map, `[]any` → Ego `[]any` array, scalars as-is).

**`remapDecodedValue` changes:**

- `case *data.Struct`: replaced the JSON-H1 single-level fix with `target.Get(k)` + `reconstructValue(v, existing)` for each field.
- `case *data.Array`: before `SetSize`, captures `origLen` and `elemModel` (first existing element). After resize, calls `reconstructValue(v, thisModel)` per element using the original element as the type hint.
- `case *data.Map`: uses `InstanceOfType(target.ElementType())` as `elemModel` (nil when interface); calls `reconstructValue(v, elemModel)` per value. The `default` case is unchanged (JSON-H2 tracked separately).

**Observable behavior change:** Unmarshaling a JSON array of objects into `[]any{}` (no struct model available) now produces Ego `*data.Map` elements instead of raw Go `map[string]any` values. The Ego type seen by callers changes from `interface{}` to `map[string]interface{}`. This is strictly better — callers can now index into the maps with normal Ego map syntax.

Test in `tests/json/unmarshal.ego`:

- `"json: Unmarshal - array of objects returns Go maps"` → renamed `"json: Unmarshal - array of objects into []any gives Ego maps"` and updated assertion from `interface{}` to `"map[string]interface{}"`.
- `"json: Unmarshal - array of structs restores element type"` added: verifies that a `[]any{pt, pt}` destination produces typed struct elements after unmarshal.

