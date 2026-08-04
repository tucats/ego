# MATH-11 — `notByteCode` and `negateByteCode` return raw (undecorated) errors from `c.Pop()`

**Affected functions:** `notByteCode`, `negateByteCode`  
**File:** `bytecode/math.go`  
**Risk:** Low — stack-underflow errors from these two functions lack the module
name and source-line annotation that all other runtime errors carry  
**Discovered by:** code review during `math_test.go` comprehensive audit  
**Status: RESOLVED**

## MATH-11: Description

Both `notByteCode` and `negateByteCode` returned raw errors from `c.Pop()` /
`c.PopWithoutUnwrapping()` without wrapping them in `c.runtimeError(err)`.

## MATH-11: Fix

Both call sites now wrap via `c.runtimeError`, matching the pattern used in all
other bytecode instruction functions and previously fixed in COMPARE-4 and LOAD-1:

```go
// notByteCode:
v, err = c.Pop()
if err != nil {
    // MATH-11 fix: decorate the error with module/line info.
    return c.runtimeError(err)
}

// negateByteCode:
v, err := c.PopWithoutUnwrapping()
if err != nil {
    // MATH-11 fix: wrap with c.runtimeError to attach source-location info.
    return c.runtimeError(err)
}
```
