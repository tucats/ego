# RANGE-1 — `rangeNextInteger` unconditionally calls `c.symbols.Set` without guarding empty or discarded variable names

**Affected function:** `rangeNextInteger`  
**File:** `bytecode/range.go`  
**Risk:** High — any for-range loop over an integer where the index variable is
discarded (`_`) or absent (`""`) fails at runtime with `ErrUnknownSymbol`
instead of silently skipping the assignment  
**Discovered by:** `Test_rangeNextInteger_DiscardedIndex_CurrentlyBroken_RANGE1`,
`Test_rangeNextInteger_EmptyIndexName_CurrentlyBroken_RANGE1`  
**Status: RESOLVED**

## RANGE-1: Description

Every `rangeNext*` helper except `rangeNextInteger` guards the index-variable
write with:

```go
if r.indexName != "" && r.indexName != defs.DiscardedVariable {
    err = c.symbols.Set(r.indexName, r.index)
}
```

`rangeNextInteger` was missing this guard, so discarded or absent index names
caused `ErrUnknownSymbol` instead of silently skipping the assignment.

## RANGE-1: Fix

Added the same guard to `rangeNextInteger`:

```go
// Only store the index when the caller declared a real variable for it.
// Skip when the name is "" (not declared) or "_" (deliberately discarded).
if r.indexName != "" && r.indexName != defs.DiscardedVariable {
    err = c.symbols.Set(r.indexName, r.index)
}
```

Tests renamed from `_CurrentlyBroken_RANGE1` to `_RANGE1`; both now assert
`nil` error and verify the full iteration completes successfully.
