# CREATE-3 — `makeArrayByteCode` element-pop loop swallowed stack underflow silently

**Affected function:** `makeArrayByteCode`  
**File:** `bytecode/create.go`  
**Risk:** Low — a compiler that emits the wrong number of Push instructions
before MakeArray would produce a silently zeroed array instead of a runtime
error  
**Discovered by:** `Test_makeArrayByteCode_ElementStackUnderflow`  
**Status: RESOLVED**

## CREATE-3: Description

The element-population loop in `makeArrayByteCode` wrapped each `c.Pop()` in an
`if err == nil` guard:

```go
for i := 0; i < count; i++ {
    if value, err := c.Pop(); err == nil {
        // ... set element ...
    }
    // If Pop() failed, the body was silently skipped — no error returned.
}
```

When the stack ran out of elements before all `count` slots were filled,
`c.Pop()` returned `ErrStackUnderflow`.  The `err == nil` condition was false,
the loop body was skipped, and the unset element retained the zero value of the
base type.  After the loop the partially-initialized array was pushed and `nil`
was returned, making the underflow completely invisible.

This was in contrast to `arrayByteCode` (the `Array` opcode), whose element loop
correctly propagated `Pop` errors immediately.

Note: a `StackMarker` in the element position was **not** affected by this bug —
`Pop()` returns a `StackMarker` successfully (no error), and
`coerceConstantArrayInitializer` then detects it with `isStackMarker` and returns
`ErrFunctionReturnedVoid`.  The silent-skip only affected genuine stack underflow.

## CREATE-3: Fix

The `if err == nil` guard was replaced with an explicit two-statement pop and
immediate error return, matching the pattern used in `arrayByteCode`:

```go
for i := 0; i < count; i++ {
    // Pop the next element value.  Any error (including stack underflow) is
    // returned immediately — CREATE-3 fix.
    value, err := c.Pop()
    if err != nil {
        return err
    }
    // ... set element ...
}
```

`Test_makeArrayByteCode_ElementStackUnderflow` now asserts `ErrStackUnderflow`
when only the base type is on the stack and count=1 requires one element pop.
