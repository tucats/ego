# INDEX-6 — Frame-pointer walks check for zero but not for the length of the stack

**Affected functions:** `(*Context).SetBreakOnReturn`, `(*Context).FormatFrames`,
`(*Context).GetFrame`
**File:** `language/bytecode/callframe.go`
**Risk:** Medium — a stack trace or debugger command can panic instead of
reporting the frames
**Status: RESOLVED**

## INDEX-6: Description

All three functions indexed the stack directly, guarded only against a zero
frame pointer:

```go
for (maxDepth < 0 || depth < maxDepth) && framePointer > 0 {
    callFrameValue := c.stack[framePointer-1]
```

`framePointer > 0` establishes only the lower bound. The walks in `FormatFrames`
and `GetFrame` chase `framePointer` through `callFrame.fp`, a value saved when
the frame was pushed, while `callFramePop` truncates `c.stack` back to the stack
pointer:

```go
c.stack = append(c.stack[:c.stackPointer], topOfStackSlice...)
```

So `len(c.stack)` shrinks over the life of a context, and a saved `fp` can name
a position that no longer exists in the slice. Because `FormatFrames` runs while
formatting an error and `SetBreakOnReturn` runs from the debugger's "step out"
command, the result is a crash in the code whose job is to explain a problem.

## INDEX-6: Fix

A single `frameAt` helper now owns the bounds check and the type assertion, and
the three callers use it. An unusable frame pointer yields nil, which stops the
walk:

```go
func (c *Context) frameAt(framePointer int) *CallFrame {
    if framePointer < 1 || framePointer > len(c.stack) {
        return nil
    }

    callFrame, ok := c.stack[framePointer-1].(*CallFrame)
    if !ok {
        return nil
    }

    return callFrame
}
```

This also consolidates the `framePointer-1` convention documented by the CALL-6
fix into one place, so a future caller cannot get the offset wrong again.
