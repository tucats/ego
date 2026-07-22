# OPTIMIZER-6 — `continue` after `found=false` in placeholder mismatch should be `break`

**Affected function:** `optimize` (inner pattern-match loop)  
**File:** `bytecode/optimizer.go`  
**Risk:** Minor performance — after a mismatch is detected, the inner loop
wastes time checking additional pattern positions  
**Status: RESOLVED**

## OPTIMIZER-6: Description

The placeholder consistency check used `continue` after setting `found = false`,
causing the inner pattern loop to keep checking more instructions even though
the match was already known to have failed.

## OPTIMIZER-6: Fix

The entire consistency-check block was simplified as part of OPTIMIZER-9 (see
below).  The resulting code uses `break` uniformly:

```go
if value.Value != i.Operand {
    found = false
    break
}
```

All three early-exit paths in the inner pattern loop now use `break`
consistently.
