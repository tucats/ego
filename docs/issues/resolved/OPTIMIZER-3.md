# OPTIMIZER-3 — No opcode-indexed dispatch: all rules tried at every position

**Affected function:** `optimize`  
**File:** `bytecode/optimizer.go`  
**Risk:** Performance — redundant work proportional to (number of rules) ×
(fraction of instructions that cannot start any pattern)  
**Status: RESOLVED**

## OPTIMIZER-3: Description

The inner loop tried every enabled optimization rule at every position,
regardless of whether the current opcode could possibly start any of those
rules.

## OPTIMIZER-3: Fix

`rulesByFirstOpcode` (a `map[Opcode][]int`) is built once before the main
loop in the same pass that computes `maxPatternSize`.  At each position, only
the rules whose first-pattern opcode matches the current instruction are
tried:

```go
candidates := rulesByFirstOpcode[b.instructions[idx].Operation]
for _, ruleIdx := range candidates {
    optimization := optimizations[ruleIdx]
    ...
}
```

The rule loop is also broken out of immediately when a match fires, so the
outer loop controls the retry position cleanly (previously the inner loop
kept running with the backed-up `idx` but was iterating a pre-filtered slice
for the old opcode).
