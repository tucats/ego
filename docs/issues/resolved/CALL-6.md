# CALL-6 — `SetBreakOnReturn` reads the wrong stack slot (off-by-one)

**Affected function:** `SetBreakOnReturn`  
**File:** `bytecode/callframe.go`  
**Risk:** Medium — the debugger "step out / break on return" feature was
silently non-functional; it logged an error but never set the flag  
**Discovered by:** `Test_SetBreakOnReturn_SetsBreakOnReturnFlag`,
`Test_SetBreakOnReturn_FrameAtFPMinusOne`  
**Status: RESOLVED**

## CALL-6: Frame-pointer convention

After `callFramePushWithTable` the runtime stack layout is:

```text
index:  ... | old_sp      | old_sp+1 ...
             | *CallFrame  | [callee locals / return values]
             | fp - 1      |
```

`c.framePointer` is set to `old_sp + 1` — one slot **past** the frame.  All
frame-reading code (`callFramePop`, `FormatFrames`, `GetFrame`) uses
`stack[framePointer-1]` to reach the `*CallFrame`.

## CALL-6: Original behavior

`SetBreakOnReturn` used `stack[framePointer]` (without the `-1`):

```go
callFrameValue := c.stack[c.framePointer]   // ← one slot too high
if callFrame, ok := callFrameValue.(*CallFrame); ok {
    callFrame.breakOnReturn = true
    c.stack[c.framePointer] = callFrame     // ← one slot too high
} else {
    ui.Log(...)   // always reached — type assertion always failed
}
```

When the callee had no local data on its stack, `stack[framePointer]` was nil.
The type assertion `nil.(*CallFrame)` failed silently, the `else` branch logged
an error, and `breakOnReturn` was **never set**.  The debugger's "step out"
command therefore had no effect.

## CALL-6: Fix

Both accesses changed from `c.framePointer` to `c.framePointer-1`, matching
the convention used everywhere else in the file:

```go
callFrameValue := c.stack[c.framePointer-1]   // was: c.framePointer
if callFrame, ok := callFrameValue.(*CallFrame); ok {
    callFrame.breakOnReturn = true
    c.stack[c.framePointer-1] = callFrame     // was: c.framePointer
} else {
    ui.Log(...)
}
```

A comment was also added to `SetBreakOnReturn` explaining the frame-pointer
convention so the same mistake is not repeated.

`Test_SetBreakOnReturn_SetsBreakOnReturnFlag` confirms that
`ctx.breakOnReturn` is `true` after `SetBreakOnReturn()` + `callFramePop()`.
`Test_SetBreakOnReturn_FrameAtFPMinusOne` confirms the slot layout invariant.
All 869 Ego-language integration tests continue to pass.

After the fix, `Test_SetBreakOnReturn_CurrentlyFailsDueToOffByOne` must be
updated to assert `ctx.breakOnReturn == true`.
