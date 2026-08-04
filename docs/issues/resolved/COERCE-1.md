# COERCE-1 — `NeedsCoerce` returns the wrong answer when the `Push` operand does not match the target type

**Affected function:** `NeedsCoerce` (method on `ByteCode`)  
**File:** `bytecode/coerce.go`  
**Risk:** Low — caused unnecessary Coerce instructions when types already
matched, and skipped needed Coerce instructions when a `Push` operand had a
different type than the target (e.g. int literal in a `[]float64` array)  
**Discovered by:** `Test_NeedsCoerce_LastInstructionPush_NonMatchingType`  
**Status: RESOLVED**

## COERCE-1: Original behavior

The `Push` branch of `NeedsCoerce` returned `data.IsType(i.Operand, kind)`,
which is `true` when the pushed value already IS the target type:

- **Matching type** → `true` → redundant Coerce emitted (no-op at runtime)
- **Non-matching type** → `false` → Coerce silently skipped; values were left
  in their original Go type rather than being converted to the array's declared
  element type

## COERCE-1: Fix

The `Push` branch now returns `!data.IsType(i.Operand, kind)`:

```go
if i.Operation == Push {
    // Coerce is needed only when the pushed value does NOT already match.
    return !data.IsType(i.Operand, kind)   // was: data.IsType(...)
}
```

`Test_NeedsCoerce_LastInstructionPush_MatchingType` now asserts `false`
(no redundant Coerce), and `Test_NeedsCoerce_LastInstructionPush_NonMatchingType`
now asserts `true` (Coerce IS emitted when types differ).

All 869 Ego-language integration tests pass in both `--types strict` and
`--types dynamic` mode after this change.
