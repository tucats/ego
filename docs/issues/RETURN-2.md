# RETURN-2: `err` from `c.Pop()` in the bool branch is silently overwritten

| | |
| :-- | :-- |
| **Affected function** | `returnByteCode` in `bytecode/return.go` |
| **Risk** | LOW — in practice the compiler only generates `Return(true)` when a value is guaranteed to be on the stack, so Pop() should never fail here |
| **Discovering test** | `Test_returnByteCode_BoolReturn_EmptyStack_RETURN2` |
| **Status** | RESOLVED |

## RETURN-2: Original behavior

In the `bool` branch the error from `c.Pop()` is captured in `err` but is never
checked before the function continues:

```go
if b, ok := i.(bool); ok && b {
    c.result, err = c.Pop()  // err set here...
    if isStackMarker(c.Result) {
        ...
    }
    c.resultSet = true
}
// ...
if c.framePointer > 0 {
    err = c.callFramePop()  // ...but overwritten here
}
```

If `c.Pop()` fails (the stack is unexpectedly empty), the error is silently
replaced by whatever `callFramePop()` returns.  In the `int(1)` sub-branch the
same issue exists, though there `err` is at least tested before the secondary
marker pop.

## RETURN-2: Fix

An explicit error check was added immediately after the Pop, combined with the
RETURN-1 correction in one contiguous block:

```go
c.result, err = c.Pop()
if err != nil {
    return c.runtimeError(err)
}
if isStackMarker(c.result) {   // c.result (field), not c.Result (method)
    return c.runtimeError(errors.ErrFunctionReturnedVoid)
}
```

`Test_returnByteCode_BoolReturn_EmptyStack_RETURN2` confirms that calling
Return(true) with an empty callee stack now returns a non-nil error instead of
silently proceeding.
