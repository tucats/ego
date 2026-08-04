# BUG-03 — Type assertions `v.(T)` always succeed regardless of actual type

**Severity:** HIGH

**Description:**  
A type assertion `v.(T)` on an `any`-typed variable always succeeds and returns
the underlying value regardless of whether the value's actual type matches `T`.
The type name `T` is ignored. The two-value form `val, ok := v.(T)` also always
returns `ok = true`.

In Go, `v.(string)` on a value containing `42` (int) panics. In Ego, it silently
returns `42` and reports `typeof` as `"string"`.

**Reproducer:**

```go
import "fmt"

func main() {
    var v any = 42

    s := v.(string)             // should panic or error
    fmt.Println("s:", s)        // prints: 42  (WRONG)
    fmt.Println("type:", typeof(s)) // prints: string  (WRONG - the int was not converted)

    s2, ok := v.(string)        // should return ("", false)
    fmt.Println("ok:", ok, "s2:", s2)  // prints: true 42  (WRONG)
}
```

**Actual output:**

```text
s: 42
type: string
ok: true s2: 42
```

**Expected output:**

```text
Error: interface conversion: interface{} is int, not string
```

or in a two-value assertion: `ok: false s2:`

**Notes:**  
Correct type assertion (`v.(int)`) does work when the type matches. The bug is
that incorrect assertions don't fail — the assertion target type is ignored and
the stored value is returned as-is.

**Resolution (June 2026):**

**Design decision:** Type assertions validate the actual stored type in **all**
type-strictness modes (strict, relaxed, and dynamic). A type assertion is not a
coercion — it asks "does this interface value actually hold a T?" rather than
"can this value be converted to T?" Cast functions (`int()`, `string()`,
`float64()`, etc.) already provide coercion. Making assertions succeed by
coercion eliminates the only runtime mechanism for asking the first question;
the `v, ok := x.(T)` form only has value if `ok` can be `false`.

**Root cause:** `unwrapByteCode` in `bytecode/types.go` had two separate code
paths based on `c.typeStrictness`:

- **Strict mode**: checked `actualType.IsType(newType)`; returned `(nil, false)`
  on mismatch. Correct.
- **Non-strict modes**: called `data.Coerce(value, newType.InstanceOf(...))`,
  unconditionally converting any value to any type and reporting success,
  ignoring the actual stored type entirely. Broken.

**Secondary bug:** the `"any"` alias for `interface{}` was not recognized. The
type lookup loop compared `td.Kind.Name()` (which returns `"interface{}"`)
against the string `"any"`, so `v.(any)` always returned `ErrInvalidType` instead
of succeeding. Fixed by normalizing `"any"` to `data.InterfaceTypeName`
(`"interface{}"`) before the loop.

**Fixes applied:**

`bytecode/types.go` — `unwrapByteCode`: the two-path `if/else` was replaced
with a unified check:

- If `newType.Kind() == data.InterfaceKind` (target is `any`): always succeeds,
  since every value satisfies the empty interface.
- If `!actualType.IsType(newType)`: push `(nil, false)` and return. Applies in
  all three type-strictness modes.
- If types match: push `(value, true)`.

`bytecode/types.go` — `unwrapByteCode` (type lookup): added normalization of
the string `"any"` to `data.InterfaceTypeName` before the `TypeDeclarations`
loop.

**Tests added:**

- `bytecode/types_test.go` — nine new Go unit tests (section 2b) covering the
  BUG-03 regression in all three modes, plus the `"any"` alias fix:
  - `Test_unwrapByteCode_BUG03_Relaxed_WrongType`
  - `Test_unwrapByteCode_BUG03_Dynamic_WrongType`
  - `Test_unwrapByteCode_BUG03_Relaxed_CorrectType`
  - `Test_unwrapByteCode_BUG03_Dynamic_CorrectType`
  - `Test_unwrapByteCode_BUG03_IntToString`
  - `Test_unwrapByteCode_BUG03_FloatToInt`
  - `Test_unwrapByteCode_BUG03_AnyAlias`
  - `Test_unwrapByteCode_BUG03_InterfaceAlias`
  - `Test_unwrapByteCode_BUG03_AllModesConsistent`

- `tests/types/type_assertions.ego` — twelve new Ego language tests covering
  correct-type and wrong-type assertions in both the single-value and two-value
  forms, all scalar types, the `any` alias, and the multi-assert type-guard
  pattern.

