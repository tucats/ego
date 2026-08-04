# OPTIMIZER-8 — `data.Int` failure in branch-check scan aborts the entire optimization pass

**Affected function:** `optimize`  
**File:** `bytecode/optimizer.go`  
**Risk:** Low correctness concern — malformed bytecode (branch with non-integer
operand) causes the entire optimization pass to fail instead of simply skipping
that branch  
**Status: RESOLVED**

## OPTIMIZER-8: Description

A malformed branch operand (non-integer) caused `optimize` to return an error
and abort the entire optimization pass.

## OPTIMIZER-8: Fix

Subsumed by OPTIMIZER-1.  When the `branchTargets` set is built, a malformed
branch operand is now logged and simply omitted from the set rather than
aborting the pass:

```go
if dest, err := data.Int(i.Operand); err == nil {
    branchTargets[dest] = true
} else {
    ui.Log(ui.OptimizerLogger, "optimizer.branch.malformed", ui.A{"operand": i.Operand})
}
```

Omitting the address is conservative: a pattern overlapping that address is
not rejected (the entry is absent), but in practice the address will never
coincide with a valid pattern window unless the bytecode is severely malformed.
