# BUG-55 — `reflect.Reflect(v).Members()`/`.Functions()` crash when called with parentheses as documented

**Severity:** MEDIUM  **Status:** Fixed

**Description:**  
`docs/LANGUAGE.md`'s `reflect.Reflect()` method table documents `Members()` and
`Functions()` as callable methods (with parentheses), alongside genuine methods like
`String()`. Calling either with parentheses, exactly as documented, fails.

**Reproducer:**

```go
import "reflect"

type Point struct {
    X int
    Y int
}

func main() {
    pt := Point{X: 1, Y: 2}
    r := reflect.Reflect(pt)
    fmt.Println("Members:", r.Members())
}
```

**Actual output:**

```text
Error: at main(line 10), invalid function invocation: ["X", "Y"]
```

(`.Functions()` fails the same way, e.g. `invalid function invocation: ["Sum() int"]`.)

**Expected output:**

```text
Members: [X, Y]
```

**Notes:**  
Root cause: in `internal/runtime/reflect/types.go`, `Members` and `Functions` are
registered via `DefineField(...)` (plain fields of `[]string` type), not `DefineFunction`.
`call.go` has a special case letting a bare **string** field be "called" with `()` and
simply return itself (used for pseudo-methods like `Type()`), but no equivalent special
case exists for array-typed fields, so calling them raises `ErrInvalidFunctionCall`.
Undocumented workaround: use `r.Members`/`r.Functions` without parentheses.

**Fix:**  
`internal/language/bytecode/call.go`'s `callByteCode` gained a second special case,
immediately following the pre-existing string one, for a bare `*data.Array` value called
with zero arguments — it is pushed back onto the stack as the "call result" exactly like
the string case does:

```go
if arr, ok := functionPointer.(*data.Array); ok && argc == 0 {
    _ = c.push(arr)

    return nil
}
```

This directly fixes both `Members()` and `Functions()` (both plain `[]string` fields on
`ReflectReflectionType`), matching the documented reproducer's expected output exactly.

**Accepted trade-off (same as the pre-existing string case):** this special case cannot
distinguish "a field access followed by `()`" from "a plain array-typed local variable
called with `()`" — both look identical by the time `callByteCode` pops the value off the
stack. So, exactly like a bare string variable called with `()` already silently returned
itself before this fix, a bare array variable called with `()` now does too, instead of
raising `ErrInvalidFunctionCall`. This mirrors the existing, accepted design for the
string case rather than introducing a new category of risk.

**Remaining, out-of-scope limitation:** a broader survey of all nine pseudo-methods
documented in `docs/LANGUAGE.md`'s `reflect.Reflect()` table found that `BaseType()`,
`Package()`, and `Type()` already work today only because their field values happen to be
strings (coincidentally hitting the pre-existing string special case). `Declaration()`
(struct-typed), `IsType()`/`Native()` (bool-typed), and `Size()` (int-typed) still raise
`ErrInvalidFunctionCall` when called with parentheses, for the same underlying reason
(no special case for those value kinds) — but this is outside BUG-55's documented scope,
which calls out array-typed fields specifically.

Regression tests: `Test_callByteCode_ArrayFunctionZeroArgs` and
`Test_callByteCode_ArrayFunctionNonZeroArgs` in
`internal/language/bytecode/call_test.go`, plus a new `@test` case in
`tests/reflect/reflect_structs.ego` confirming `.Members()`/`.Functions()` (called with
parens) agree exactly with the pre-existing bare-field-access workaround.

