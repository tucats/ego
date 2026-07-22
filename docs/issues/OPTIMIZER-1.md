# OPTIMIZER-1 — Branch-target scan is O(n²): pre-build a target set instead

**Affected function:** `optimize`  
**File:** `bytecode/optimizer.go`  
**Risk:** Performance — for large bytecode bodies the optimizer is dominated by
this scan; it is the primary reason optimizer mode 1 (conditional) skips
short sequences  
**Status: RESOLVED**

## OPTIMIZER-1: Description

Inside the main `optimize` loop, for every position `idx` and every
optimization rule, the code performed a linear scan over **all** instructions
to check whether any branch instruction targeted an address inside the candidate
pattern window — O(n²m) total cost.

## OPTIMIZER-1: Fix

A `map[int]bool` of branch target addresses (`branchTargets`) is now built
lazily: it is populated before the first outer-loop iteration and rebuilt
after every `Patch` call (which adjusts branch operands throughout the stream).
A `needsRebuild` flag controls when the rebuild fires.

Inside the rule loop, the branch-target check is now O(patternLen):

```go
for offset := 0; offset < len(optimization.Pattern); offset++ {
    if branchTargets[idx+offset] {
        found = false
        break
    }
}
```

Malformed branch operands (non-integer) are logged and skipped instead of
aborting the pass (subsumed from OPTIMIZER-8).
