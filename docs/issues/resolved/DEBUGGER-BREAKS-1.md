# DEBUGGER-BREAKS-1 — Hit tracking lost in evaluationBreakpoint range loop

**File:** `debugger/breaks.go`  
**Function:** `evaluationBreakpoint`  
**Risk:** High — conditional breakpoints (`break when <expr>`) fire on every
matching step instead of once per "becoming true" edge  
**Status: RESOLVED**

## DEBUGGER-BREAKS-1: Original behavior

`evaluationBreakpoint` iterates over `breakPoints` with a `for _, b := range`
loop.  In Go, the range loop copies each element into the loop variable `b`
before executing the body.  Any changes to `b` inside the loop modify only
that **copy** — the original element in the slice is untouched.

Two `b.hit` mutations inside the loop were therefore silently lost:

```go
// BreakValue case (conditional breakpoint):
if b.hit > 0 {
    break          // intended to suppress re-trigger — but b.hit is always 0!
}
...
if prompt {
    b.hit++        // increments the copy; breakPoints[n].hit stays 0
} else {
    b.hit = 0      // also a no-op on the original
}

// BreakAlways case (line breakpoint):
b.hit++            // hit-count statistics never increment
```

**Consequence for `BreakValue`:** The guard `if b.hit > 0` is designed to
suppress a conditional breakpoint from re-triggering on every subsequent step
once it has first fired (the user must resume before it fires again).  Because
`b.hit` is always reset to 0 by the copy semantics, the guard never activates.
Every single source line where the condition evaluates to true causes another
debugger stop, making programs with conditional breakpoints nearly unusable.

**Consequence for `BreakAlways`:** The hit counter statistics in the slice are
never updated, so any future `show breaks` display showing hit counts would
always show 0.

## DEBUGGER-BREAKS-1: Fix

Changed the loop from `for _, b := range breakPoints` to
`for n := range breakPoints` and used `breakPoints[n]` directly for all reads
and writes.  This ensures that `hit` changes are applied to the real slice
element, not a temporary copy.

```go
for n := range breakPoints {
    switch breakPoints[n].Kind {
    case BreakValue:
        if breakPoints[n].hit > 0 {
            break
        }
        ...
        if prompt {
            breakPoints[n].hit++
        } else {
            breakPoints[n].hit = 0
        }
    case BreakAlways:
        ...
        breakPoints[n].hit++
    }
}
```
