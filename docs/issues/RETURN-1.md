# RETURN-1: `isStackMarker(c.Result)` uses the bound method instead of the field — guard never fires

| | |
| :-- | :-- |
| **Affected function** | `returnByteCode` in `bytecode/return.go` |
| **Risk** | MEDIUM — a StackMarker on the stack where a return value is expected is silently propagated to the caller instead of raising an error |
| **Discovering test** | `Test_returnByteCode_BoolReturn_StackMarker_ReturnsVoidError_RETURN1` |
| **Status** | RESOLVED |

## RETURN-1: Original behavior

In the `bool` branch of `returnByteCode`, after popping the return value, there
is a guard that is supposed to detect when a StackMarker was returned (which
means a function that returns void was incorrectly used as an expression):

```go
c.result, err = c.Pop()
if isStackMarker(c.Result) {          // ← RETURN-1 bug
    return c.runtimeError(errors.ErrFunctionReturnedVoid)
}
c.resultSet = true
```

`c.Result` (uppercase `R`) is the **bound method expression** — it evaluates to
a value of type `func() any`, not the `any` stored in `c.result`.

```text
c.result   — the unexported field of type any; holds the popped value
c.Result   — the exported method func (c *Context) Result() any
c.Result   — WITHOUT parentheses: a bound method of type func() any
```

`isStackMarker` checks whether its argument is a `StackMarker` struct or a
`*CallFrame`.  A bound method `func() any` is neither, so `isStackMarker`
always returns `false`.  The guard is dead code.

When a StackMarker is on the stack:

- It is popped and stored in `c.result` (the field).
- `c.resultSet` is set to `true`.
- `callFramePop` re-pushes the StackMarker onto the caller's stack.
- The caller receives a StackMarker as a "value", which can corrupt subsequent
  operations.

## RETURN-1: Fix

Changed `c.Result` (method reference) to `c.result` (field):

```go
c.result, err = c.Pop()
if err != nil {                        // RETURN-2 fix applied together
    return c.runtimeError(err)
}
if isStackMarker(c.result) {           // ← corrected (was c.Result)
    return c.runtimeError(errors.ErrFunctionReturnedVoid)
}
```

`Test_returnByteCode_BoolReturn_StackMarker_ReturnsVoidError_RETURN1` confirms
that pushing a StackMarker and calling Return(true) now returns
`ErrFunctionReturnedVoid`.
