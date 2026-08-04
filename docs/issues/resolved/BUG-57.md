# BUG-57 — `@type relaxed` does not coerce assigned values to the receiving variable's type

**Severity:** MEDIUM  **Status:** Fixed

**Description:**  
`docs/LANGUAGE.md` documents that in `relaxed` type mode, "if possible a value will be
converted before being stored to match the type of the receiving variable." In practice,
assigning a value of a different (but coercible) type in relaxed mode simply changes the
variable's type to match the assigned value, exactly like `dynamic` mode — no coercion to
the original declared type occurs. Separately, `@type strict` mode silently *truncates*
numeric values on assignment without erroring, even though the documentation states strict
mode disallows storing a value of a different type — string values are correctly rejected
in strict mode, but numeric widening/narrowing is not, which is an inconsistent application
of the same rule.

**Reproducer:**

```go
func main() {
    @type relaxed
    {
        var w int = 5
        fmt.Println("before:", typeof(w))
        w = 3.7
        fmt.Println("after:", w, typeof(w))
    }
}
```

**Actual output:**

```text
before: int
after: 3.7 float64
```

**Expected output:**

```text
before: int
after: 3 int
```

**Notes:**  
The same assignment under `@type strict` produces `w = 3, int` with no error at all
(confirmed strict mode *does* correctly reject `w = "hello"` for the same `int` variable
with `invalid type for this variable`), so the enforcement is inconsistently applied within
strict mode itself, independent of the `relaxed`-mode issue described above.

**Fix:**  
Root cause: `Context.checkType` (`internal/language/bytecode/context.go`) used a single
`canCoerce` boolean for two unrelated conditions — "the incoming value was a compile-time
literal constant" (numeric/string/bool literals are pushed wrapped in `data.Immutable` by
the `Constant` opcode) and "the destination variable currently holds an interface value."
In `relaxed` mode, `canCoerce == true` triggered an *immediate, unconditional* early return
before any coercion was attempted — so *any* literal assigned to *any* typed variable
bypassed coercion entirely and just changed the variable's type, exactly like `dynamic`
mode. This only affected literals; a non-constant expression of a mismatched, non-numeric
type would still have fallen through to the final `reflect.TypeOf` comparison and (correctly
by accident) errored — but the bug report's own reproducer assigns a bare float literal, so
it hit the early-return path. Separately in `strict` mode, the same conflated flag caused the
numeric-coercion sub-branch to run for *any* literal, silently truncating (`3.7 → 3`) with no
loss-of-precision check, while non-numeric literals (e.g. `"hello"`) fell through to the
explicit type-mismatch check and correctly errored — hence the inconsistency the notes above
describe.

`checkType` was rewritten to implement the three modes as three genuinely distinct policies,
matching Go's own assignability rules for `strict`:

- **dynamic** (`NoTypeEnforcement`): unchanged — no check at all, variables freely change type.
- **relaxed** (`RelaxedTypeEnforcement`): the incoming value — literal or not — is now always
  run through `data.Coerce(value, existingValue)` when its concrete type differs from the
  existing variable's type, so the variable's type never changes. An error is returned only
  when `data.Coerce` itself reports the value is genuinely inconvertible (e.g. storing
  `"not a number"` into an `int` variable). A value bound for an `interface{}`-typed
  destination is still stored as-is, since holding any concrete type is legal for an
  interface and isn't a type change.
- **strict** (`StrictTypeEnforcement`): a non-constant value of a different type is always
  rejected — the caller must cast explicitly, matching Go's requirement that mixed-type
  expressions be explicit. A literal constant may still convert implicitly between numeric
  kinds, but only when the conversion loses no information: the coerced value is round-tripped
  back through `data.Float64` and compared against the original, and a mismatch (e.g.
  `3.7 → 3`) is rejected with `ErrLossOfPrecision`, independent of the
  `ego.runtime.precision.error` setting (that setting governs explicit runtime casts, not this
  implicit-constant check, which must behave the same regardless of server configuration).
  This mirrors Go's own untyped-constant convertibility rule: `var f float64 = 5` and
  `w = 3` succeed (exact), `w = 3.7` fails (lossy) even though both are constants, and
  `w = someFloatVariable` fails even when the runtime value happens to be exactly `3.0`,
  because it isn't a constant.

Verified against the documented reproducer (now prints `after: 3 int`, not
`after: 3.7 float64`) plus the additional strict-mode cases from the notes above. Regression
tests: `Test_Context_checkType_RelaxedMode_CoercesConstant_KeepsType`,
`Test_Context_checkType_RelaxedMode_CoercesVariable_KeepsType`,
`Test_Context_checkType_RelaxedMode_Uncoercible_Error`,
`Test_Context_checkType_StrictMode_LossyConstant_Error`,
`Test_Context_checkType_StrictMode_LosslessConstant_OK`, and
`Test_Context_checkType_StrictMode_NonConstantCrossType_Error` in
`internal/language/bytecode/context_test.go`, plus five new `@test` cases in
`tests/types/type_enforcement.ego` exercising all three modes end-to-end.

