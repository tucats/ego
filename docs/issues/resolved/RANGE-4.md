# RANGE-4 — `rangeNextByteCode` returns a raw error for the `[]any` case without `c.runtimeError()` decoration

**Affected function:** `rangeNextByteCode`  
**File:** `bytecode/range.go`  
**Risk:** Low — the `[]any` case is dead code for well-formed programs; but if
reached, the error lacks module and source-line information  
**Discovered by:** `Test_rangeNextByteCode_SliceAnyValue_CurrentlyBroken_RANGE4`  
**Status: RESOLVED**

## RANGE-4: Description

The `[]any` case returned a raw error without `c.runtimeError()` decoration,
losing module and source-line context inconsistently with the rest of the package.

## RANGE-4: Fix

```go
// Before:
case []any:
    return errors.ErrInvalidType.Context("[]any")

// After:
case []any:
    return c.runtimeError(errors.ErrInvalidType)
```

The `[]any` branch is retained as a defensive guard (with an updated comment
explaining it is unreachable in well-formed programs since `rangeInitByteCode`
already rejects `[]any` values before they can reach the range stack).

Test renamed from `_CurrentlyBroken_RANGE4` to `_RANGE4`.
