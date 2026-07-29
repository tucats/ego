# INDEX-5 — `stackCheckByteCode` starts its marker scan above the top of the stack

**Affected function:** `stackCheckByteCode`
**File:** `language/bytecode/stack.go`
**Risk:** Medium — panics when the stack slice is exactly full
**Status: RESOLVED**

## INDEX-5: Description

```go
if count, err := data.Int(i); err != nil || c.stackPointer <= count {
    return c.runtimeError(errors.ErrReturnValueCount)
} else {
    for i := c.stackPointer - (count - 1); i >= 0; i-- {
        v := c.stack[i]
```

The scan begins at `stackPointer - (count - 1)`. For `count == 1` that is
`stackPointer` itself, and for `count == 0` it is `stackPointer + 1` — both at or
above the top of the stack.

`push()` grows `c.stack` in chunks and `callFramePop` truncates it back to the
stack pointer with `c.stack = append(c.stack[:c.stackPointer], ...)`, so
`stackPointer == len(c.stack)` is a reachable state. The scan then reads one or
more entries past the end of the slice and panics.

The count is the instruction's operand, taken from the bytecode stream, so it is
not a trustworthy bound.

## INDEX-5: Fix

The starting index is clamped to the topmost live stack entry before the scan:

```go
start := c.stackPointer - (count - 1)
if start > c.stackPointer-1 {
    start = c.stackPointer - 1
}

if start >= len(c.stack) {
    start = len(c.stack) - 1
}
```
