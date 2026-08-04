# LOAD-1 — `explodeByteCode` returned raw error from `c.Pop()` without `c.runtimeError` decoration

**Affected function:** `explodeByteCode`  
**File:** `bytecode/load.go`  
**Risk:** Low — stack-underflow errors from `explodeByteCode` lacked the
module name and source line that all other runtime errors carry; correctness
was not affected  
**Discovered by:** `Test_explodeByteCode_StackUnderflow` in `bytecode/load_test.go`  
**Status: RESOLVED**

## LOAD-1: Original behavior

Every error returned by a bytecode instruction function is expected to be
decorated via `c.runtimeError(err)`, which attaches the current module name
and source line before returning the error to the caller.  This annotation
lets the Ego runtime (and the user-facing stack trace) identify exactly where
in the program the error occurred.

`explodeByteCode` returned the raw error from `c.Pop()` directly:

```go
// Original (buggy):
v, err = c.Pop()
if err != nil {
    return err   // ← raw; no module/line annotation
}
```

When the stack was empty, `c.Pop()` returned `ErrStackUnderflow`.  The error
reached the caller without any location information, inconsistent with every
other error path in `explodeByteCode` and the rest of the bytecode package.

This is the same pattern documented in COMPARE-4 for the comparison operators.

## LOAD-1: Fix

The `return err` was changed to `return c.runtimeError(err)`:

```go
// Fixed:
v, err = c.Pop()
if err != nil {
    return c.runtimeError(err)   // ← decorated with module/line
}
```

`Test_explodeByteCode_StackUnderflow` in `bytecode/load_test.go` confirms
that `ErrStackUnderflow` is returned when the stack is empty and documents
the expected behavior after the fix.
