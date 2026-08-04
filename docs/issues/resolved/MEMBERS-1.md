# MEMBERS-1 — `memberByteCode` returns raw errors from `c.Pop()` without `c.runtimeError` decoration

**Affected function:** `memberByteCode`  
**File:** `bytecode/member.go`  
**Risk:** Low — stack-underflow errors from the two Pop calls in `memberByteCode`
lack the module name and source-line annotation that all other runtime errors carry  
**Discovered by:** `Test_memberByteCode_StackUnderflow_EmptyStack`,
`Test_memberByteCode_StackUnderflow_OperandButEmptyStack`  
**Status: RESOLVED**

## MEMBERS-1: Description

`memberByteCode` made two `c.Pop()` calls — one for the member name (when the
operand is nil) and one for the object.  Both error paths returned the raw error
without `c.runtimeError()` wrapping, so stack-underflow errors lacked module/line
info.  This is the same pattern fixed in MATH-11, COMPARE-4, and LOAD-1.

## MEMBERS-1: Fix

Both error paths now use `c.runtimeError()`:

```go
// name pop — MEMBERS-1 fix:
v, err = c.Pop()
if err != nil {
    return c.runtimeError(err)
}

// object pop — MEMBERS-1 fix:
m, err = c.Pop()
if err != nil {
    return c.runtimeError(err)
}
```
