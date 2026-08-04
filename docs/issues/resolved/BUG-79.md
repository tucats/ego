# BUG-79 — Three `strings` functions declared with a return type that doesn't match their actual behavior

**Severity:** LOW

**Description:**  
Found during a documentation review of the `strings` package. Three functions in
`internal/runtime/strings/types.go` had a `Declaration.Returns` that did not match what the
function actually returns at runtime:

- `EqualFold` (a native passthrough to Go's `strings.EqualFold`, which returns `bool`) was
  declared `Returns: []*data.Type{data.StringType}`.
- `Ints` (`extractInts` in `conversion.go`, which always returns a `*data.Array` of `int`) was
  declared `Returns: []*data.Type{data.IntType}` — a bare scalar, not an array.
- `Substitution` (`substitution` in `substrings.go`, which always returns
  `data.NewList(text, err)`) was declared `Returns: []*data.Type{data.StringType}` — a single
  value, with no error.

None of these caused an observably wrong *value* at a call site: native dispatch converts
whatever the real Go function returned via reflection, independent of the declaration, and a
`data.List` result is pushed however many items it actually contains, independent of the declared
count. Both `v := strings.Ints("abc")` and `v, err := strings.Substitution(...)` already worked
correctly before this fix. The bug was purely in the declared metadata: `reflect.Type()` on the
function value reported the wrong signature (e.g. `funcEqualFold(a string, b string) string`
instead of `... bool`), which would mislead any code or documentation relying on introspection.

**Fix:**  
Corrected the three declarations: `EqualFold` → `data.BoolType`; `Ints` → `data.ArrayType(data.IntType)`;
`Substitution` → `[]*data.Type{data.StringType, data.ErrorType}`. Added regression tests asserting
`string(reflect.Type(fn))` for each in `tests/strings/search.ego`, `tests/strings/split_join.ego`,
and `tests/strings/format.ego`.

