# OPTIMIZER-5 — `Patch` corrupts the instruction array when the replacement is longer than the deleted region

**Affected function:** `Patch`  
**File:** `bytecode/optimizer.go`  
**Risk:** Latent correctness — no current optimization triggers this; all rules
shrink or preserve the instruction count  
**Status: RESOLVED**

## OPTIMIZER-5: Description

The old two-append splice pattern corrupted the instruction-array tail when
`len(insert) > deleteSize`, because the first append would overwrite positions
`start+deleteSize` and beyond before the second append captured that tail.

## OPTIMIZER-5: Fix

`Patch` now explicitly copies the tail into a fresh slice before any
appending, then assembles the final slice from scratch:

```go
tail := make([]instruction, b.nextAddress-tailStart)
copy(tail, b.instructions[tailStart:b.nextAddress])

instructions := make([]instruction, 0, newLen)
instructions = append(instructions, b.instructions[:start]...)
instructions = append(instructions, insert...)
instructions = append(instructions, tail...)
```

This is safe for any relative sizes of `insert` and `deleteSize`.
