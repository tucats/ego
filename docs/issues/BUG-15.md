# BUG-15 — `append()` to typed array silently accepts wrong-type elements

**Severity:** MEDIUM  
**Status:** Fixed (APPEND-2)

**Description:**  
`append()` did not enforce the element type of a typed array in dynamic (default)
mode. Appending a `string` to a `[]int` silently succeeded and corrupted the
array's type contract.

**Root cause:**  
`builtins/append.go` gated the element-type check with
`typeChecking < defs.NoTypeEnforcement`. In dynamic mode (`NoTypeEnforcement = 2`),
`2 < 2` is false, so the check never ran and any value was accepted.

**Fix:**  
Removed the `typeChecking < defs.NoTypeEnforcement` guard from the element-type
check (APPEND-2 in `internal/builtins/append.go`). A typed array's element-type
contract is now always enforced regardless of the type-checking mode. The mode
still controls *how* a mismatch is handled:

- **strict (0)**: reject immediately with `ErrWrongArrayValueType`
- **relaxed (1)**: attempt coercion; error if coercion fails
- **dynamic (2)**: same as relaxed — coerce if possible, error if not

Interface-typed arrays (`[]interface{}`) continue to accept any element.

**Behavior after fix:**

```go
a := []int{1, 2, 3}
a = append(a, "hello")   // error: wrong type for element of []int (all modes)
a = append(a, true)      // succeeds: bool→int coercion (relaxed/dynamic)
f := []float64{1.1}
f = append(f, 3)         // succeeds: int→float64 coercion (relaxed/dynamic)
```

**Files changed:**

- `internal/builtins/append.go` — removed `typeChecking < defs.NoTypeEnforcement`
  gate; added APPEND-2 comment block explaining the invariant
- `internal/builtins/append_test.go` — renamed `Test_Append_NoTypeEnforcementSkipsCheck`
  to `Test_Append_DynamicModeCoercesCompatibleType` (the old test documented wrong
  behavior); added `Test_Append_DynamicModeRejectsIncompatibleType` (BUG-15 fix test)
- `tests/types/append_type_check.ego` — new Ego-level tests covering: compatible
  appends, string→int rejection, coercible types, interface arrays, multi-element

