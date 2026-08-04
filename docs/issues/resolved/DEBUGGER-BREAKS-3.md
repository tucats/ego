# DEBUGGER-BREAKS-3 — clearBreak functions continue iterating after first deletion

**File:** `debugger/breaks.go`  
**Functions:** `clearBreakWhen`, `clearBreakAtLine`  
**Risk:** Medium — if duplicate breakpoints ever exist (due to a future bug),
only the first match is cleanly removed and subsequent matches may be skipped
due to slice-shift confusion  
**Status: RESOLVED**

## DEBUGGER-BREAKS-3: Original behavior

Both clear functions used `for n, b := range breakPoints` and, after finding a
match, modified `breakPoints` in-place with `append(breakPoints[:n],
breakPoints[n+1:]...)`.  This shifts every element after position `n` one
slot to the left in the backing array.

The `range` loop captured the original slice header (pointer + length) at the
start of the loop.  After the shift, element `n+1` in the backing array now
holds what was originally element `n+2`.  On the next loop iteration
`n+1` is visited — but the element originally at that position has already
moved to `n`, so it is **skipped**.

In practice, `breakAtLine` and `breakWhen` both check for an existing
breakpoint before adding, so duplicate entries should not exist.  The bug is
therefore latent.  However, failing to `break` after the first (and only
expected) deletion leaves the loop running over stale data.

## DEBUGGER-BREAKS-3: Fix

Added a `return` statement immediately after each deletion path so the
function exits as soon as the matching breakpoint is removed.  This is
consistent with the invariant that breakpoints are unique and avoids any
further iteration over the shifted slice.

```go
func clearBreakWhen(text string) {
    for n, b := range breakPoints {
        if b.Kind == BreakValue && b.Text == text {
            if len(breakPoints) == 1 {
                breakPoints = []breakPoint{}
            } else if n == len(breakPoints)-1 {
                breakPoints = breakPoints[:n]
            } else {
                breakPoints = append(breakPoints[:n], breakPoints[n+1:]...)
            }
            return  // ← added
        }
    }
}
```

(Same change applied to `clearBreakAtLine`.)
