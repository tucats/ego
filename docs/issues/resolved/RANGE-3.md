# RANGE-3 — Map remains readonly if a for-range loop exits before exhaustion

**Affected functions:** `rangeInitByteCode`, `rangeNextMap`, `popScopeByteCode`  
**Files:** `bytecode/range.go`, `bytecode/symbols.go`  
**Risk:** Medium — a map used inside a for-range loop cannot be modified for
the rest of the function scope if the loop body exits early via `break` or
`return`  
**Discovered by:** `Test_rangeNextMap_EarlyExitLeavesMapReadonly_RANGE3`  
**Status: RESOLVED**

## RANGE-3: Description

`rangeInitByteCode` locked the map readonly at the start of iteration.
`rangeNextMap` unlocked it only when the iterator was exhausted.  An early
exit via `break` or `return` bypassed the exhaustion path, leaving the map
permanently locked within the function scope.

## RANGE-3: Fix

Implemented Option B (PopScope awareness) with three coordinated changes:

**1. `rangeDefinition` — new fields** (`bytecode/range.go`):

```go
type rangeDefinition struct {
    ...
    scopeDepth int    // c.blockDepth at the time RangeInit ran
    cleanup    func() // called once when this entry is retired
}
```

A `release()` method calls cleanup exactly once (nil-clears it after the first
call to prevent double-release).

**2. `rangeInitByteCode`** — for maps, store a cleanup closure:

```go
case *data.Map:
    r.keySet = actual.Keys()
    actual.SetReadonly(true)
    r.cleanup = func() { actual.SetReadonly(false) }  // ← new
```

`r.scopeDepth = c.blockDepth` is set for all value types before pushing.

**3. `rangeNextMap`** — call `r.release()` instead of `actual.SetReadonly(false)`:

```go
if r.index >= len(r.keySet) {
    c.programCounter = destination
    c.rangeStack = c.rangeStack[:stackSize-1]
    r.release()   // ← was: actual.SetReadonly(false)
}
```

**4. `popScopeByteCode`** (`bytecode/symbols.go`) — release range entries
belonging to the scope just popped:

```go
c.blockDepth--

// Release range entries whose scope was just popped (RANGE-3 fix).
for len(c.rangeStack) > 0 && c.rangeStack[len(c.rangeStack)-1].scopeDepth > c.blockDepth {
    c.rangeStack[len(c.rangeStack)-1].release()
    c.rangeStack = c.rangeStack[:len(c.rangeStack)-1]
}
```

This fires on both `break` (which jumps to the code just before `PopScope`)
and normal exhaustion (where `release()` is a no-op because cleanup was
already called by `rangeNextMap`).

Test renamed from `_EarlyExitLeavesMapReadonly_RANGE3` to
`_EarlyExitReleasesMap_RANGE3`; now asserts the map is writable after
`popScopeByteCode` is called.
