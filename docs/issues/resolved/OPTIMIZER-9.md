# OPTIMIZER-9 — Dead `else if` condition in placeholder consistency check

**Affected function:** `optimize`  
**File:** `bytecode/optimizer.go`  
**Risk:** None — code clarity only; the condition is always true for real
bytecode  
**Status: RESOLVED**

## OPTIMIZER-9: Description

The placeholder consistency check contained a dead `else if` whose condition
(`i.Operand != sourceInstruction.Operand`) is always true for real bytecode
because real operands are never `placeholder` structs.

## OPTIMIZER-9: Fix

The entire `if value.Value == … { } else if … { }` chain was replaced with a
direct negation, and `continue` was changed to `break` (OPTIMIZER-6):

```go
if value.Value != i.Operand {
    found = false
    break
}
```

The `inMap` true-branch now has a single clear code path: if the previously
captured value matches the current operand, we continue silently; if not, we
reject the match and short-circuit.
