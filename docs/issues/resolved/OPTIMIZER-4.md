# OPTIMIZER-4 — Backtracking by `maxPatternSize` instead of the matched pattern size

**Affected function:** `optimize`  
**File:** `bytecode/optimizer.go`  
**Risk:** Performance — after a small pattern fires, the scanner may revisit
instructions that were already checked  
**Status: RESOLVED**

## OPTIMIZER-4: Description

After a successful substitution the scanner originally backed up by
`maxPatternSize + 1` (extra -1 in the formula plus the loop's `idx++`),
which was one step more than necessary.

## OPTIMIZER-4: Fix and correction to proposed approach

The proposed fix (retreat by matched pattern length) is not safe in general:
a rule with `maxPatternSize` instructions can start up to `maxPatternSize - 1`
positions *before* the match and overlap the replacement.  Using the smaller
matched length would miss those opportunities.

The correct minimum retreat is `maxPatternSize - 1`.  The old code used
`maxPatternSize` (one extra step).  The formula was changed to:

```go
idx -= maxPatternSize - 1
if idx < -1 {
    idx = -1  // after loop's idx++, resumes at 0
}
```

This is the exact minimum: a `maxPatternSize`-instruction rule starting at
`idx - (maxPatternSize - 1)` is the earliest rule that could overlap the new
replacement.  Combined with OPTIMIZER-3 (opcode dispatch), the extra iterations
at re-examined positions are near-free anyway.
