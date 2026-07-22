# STACK-2 — `readStackByteCode` guard uses `>` instead of `>=`, causing panics on boundary indices

**Affected function:** `readStackByteCode`  
**File:** `bytecode/stack.go`  
**Risk:** High — two boundary conditions trigger an unrecoverable runtime panic
(negative slice index) instead of returning `ErrStackUnderflow`  
**Discovered by:** `Test_readStackByteCode_EmptyStack_CurrentlyBroken_STACK2`,
`Test_readStackByteCode_IndexBeyondStack_CurrentlyBroken_STACK2`  
**Status: RESOLVED**

## STACK-2: Description

The guard `idx > c.stackPointer` missed the boundary case where
`idx == c.stackPointer`, causing `c.stack[(stackPointer-1)-idx]` to compute a
negative slice index and panic.

## STACK-2: Fix

Changed the guard from `>` to `>=`:

```go
// was: if idx > c.stackPointer {
if idx >= c.stackPointer {
    return c.runtimeError(errors.ErrStackUnderflow)
}
```

An expanded function-level comment was added explaining the correctness
requirement: `idx` must be strictly less than `stackPointer` so the computed
slice index `(stackPointer-1)-idx` is always non-negative.

Both tests were renamed (dropping `_CurrentlyBroken_`) and updated to assert
`ErrStackUnderflow` directly rather than catching a recover() panic:

- `Test_readStackByteCode_EmptyStack_STACK2`
- `Test_readStackByteCode_IndexEqualsStackPointer_STACK2`
