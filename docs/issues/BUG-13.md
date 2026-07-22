# BUG-13 — `typeof()` result incompatible with `switch` case matching

**Severity:** MEDIUM  
**Status:** Fixed (EQUAL-4 / COMPARE-4)

**Description:**  
`typeof(v)` returns a type object (`*data.Type`). A "cheat" in `equalByteCode` allowed
`typeof(v) == "int"` to return `true` by comparing `actual.String()` to the string literal
via `equalTypes()`. This predated the Ego type system and created an inconsistency: `==`
coerced the type to a string, but `switch` case matching did not — it tried to coerce
`"int"` (the case value) to an integer, failing with `"invalid integer value: int"`.

**Root cause:**  
`equalTypes()` in `internal/language/bytecode/equal.go` accepted a `string` v2 argument and
compared `actual.String() == v`, enabling type-to-string equality. The compiler pushes the
case value first and loads the switch expression second, so in a `switch typeof(n)` with
`case "int":`, v1 = `"int"` (string) and v2 = `*data.Type`. This reversed order went to
`genericEqualCompare` → `Normalize` → `Coerce`, which tried to coerce `"int"` into an
integer — producing the error.

**Fix:**  
Removed the string branch from `equalTypes()`. A `*data.Type` is now only equal to another
`*data.Type` with the same canonical name (`actual.String() == v.String()`), and unequal to
all non-type values. Added a symmetric guard in both `equalByteCode` and `notEqualByteCode`
to handle the case where v2 is a `*data.Type` and v1 is not (the switch-case ordering).

**Behavior after fix:**

- `typeof(n) == int` → `true` (type constant comparison — correct)
- `typeof(n) == "int"` → `false` (type is never equal to a string literal)
- `typeof(n) != "int"` → `true`
- `switch typeof(n) { case int: ... }` → matches correctly
- `switch typeof(n) { case "int": ... }` → does not match (no error)

**Callers updated:**  
Test files that compared `reflect.Reflect(v).BaseType == "typename"` (a `*data.Type` field
compared to a string literal via the cheat) were updated to either use a type constant
(`== int`, `== float32`, etc.) or a string cast (`string(r.BaseType) == "struct{...}"`).

**Files changed:**

- `internal/language/bytecode/equal.go` — removed string branch from `equalTypes()`,
  added EQUAL-4 guard for type-on-right ordering
- `internal/language/bytecode/notEqual.go` — added COMPARE-4 guard and `case *data.Type:`
- `internal/language/bytecode/equal_test.go` — updated tests for new behavior
- `tests/json/unmarshal.ego` — `reflect.Type(arr[0]) == "map[...]"` → string cast
- `tests/reflect/reflect_scalars.ego` — `.BaseType == "typename"` → type constants
- `tests/reflect/reflect_structs.ego` — `.BaseType == "struct"` → `string()` cast or `Index`
- `tests/reflect/reflect_packages.ego` — `.BaseType == "struct"` → `string()` cast or `Index`
- `tests/datamodel/float32.ego` — `.BaseType == "float32"` → `== float32` type constant
- `tests/typeof/comparison.ego` — new tests verifying the corrected behavior

