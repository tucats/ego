# CONTEXT-2 — `SetDebug` unconditionally sets `singleStep = true` regardless of argument

**Affected function:** `SetDebug`  
**File:** `bytecode/context.go`  
**Risk:** Low — no runtime correctness impact (singleStep has no effect when
`debugging` is false), but semantically unexpected and masks explicit calls
to `SetSingleStep(false)`  
**Discovered by:** `Test_Context_SetDebug_False_ClearsBothFlags`  
**Status: RESOLVED**

## CONTEXT-2: Description

`SetDebug(b)` set `c.debugging = b` but always assigned `c.singleStep = true`,
regardless of `b`:

```go
func (c *Context) SetDebug(b bool) *Context {
    c.debugging = b
    c.singleStep = true   // ← unconditional; was the bug
    return c
}
```

Calling `SetDebug(false)` left `singleStep` enabled.  If a caller explicitly
disabled step mode with `SetSingleStep(false)` and then called `SetDebug(false)`
(e.g., to temporarily pause the debugger), `singleStep` was silently reset to
`true`.  Re-enabling debugging with `SetDebug(true)` would always start in step
mode, making it impossible to re-enter the debugger in run-free (non-step) mode
without an explicit `SetSingleStep(false)` call immediately afterward.

## CONTEXT-2: Fix

`singleStep` is now assigned the same value as `b`, mirroring the pattern of
every other boolean setter in the file:

```go
func (c *Context) SetDebug(b bool) *Context {
    c.debugging = b
    c.singleStep = b   // enable step mode when debugging on; clear when off
    return c
}
```

`Test_Context_SetDebug_True_SetsBothFlags` confirms that `SetDebug(true)` sets
both `debugging` and `singleStep` to `true`.
`Test_Context_SetDebug_False_ClearsBothFlags` confirms that `SetDebug(false)`
clears both fields.
