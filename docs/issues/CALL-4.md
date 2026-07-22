# CALL-4 — `parentTable` nil guard is dead code for non-literal named functions

**Affected function:** `callBytecodeFunction`  
**File:** `bytecode/callBytecodeFunction.go`  
**Risk:** Low — no crash occurs; the behavior is correct despite the dead guard  
**Discovered by:** `Test_callBytecodeFunction_NilFindNextScope_StillSucceeds`  
**Status: RESOLVED**

## CALL-4: Original behavior

`callBytecodeFunction` guarded `parentTable` against nil inside the
`functionSymbols == nil` block using a raw, uninitialized struct literal:

```go
if parentTable == nil {
    parentTable = &symbols.SymbolTable{Name: "<none>"}  // raw struct — nil maps/values
}
```

The guard was never meaningful: for the `callFramePush` path `parentTable`
was not consumed, and for the captured-scope path `capturedScope != nil`
guaranteed `parentTable` was already non-nil.  Meanwhile the package-method
path (`functionSymbols != nil`) had no guard at all, so `Clone(nil)` could
be called when `FindNextScope` returned nil from a root context.

## CALL-4: Fix

The entire `getPackageSymbols()` call and the associated `functionSymbols`
branch were removed from `callBytecodeFunction` (see the broader CALL-4/CALL-5
investigation note below).  The dead nil guard and the raw struct literal
disappeared with the code they guarded.

The log statement is now safe by computing `parentName` conditionally:

```go
parentName := "<none>"
if parentTable != nil {
    parentName = parentTable.Name
}
```

`Test_callBytecodeFunction_NilFindNextScope_StillSucceeds` confirms that a
named function call succeeds from a root-level context (nil `FindNextScope`).

## CALL-4 / CALL-5: Combined investigation note

Fixing CALL-5 (making `getPackageSymbols()` correctly return the package's
embedded symbol table) exposed a deeper issue: the `functionSymbols != nil`
clone path in `callBytecodeFunction` broke global scope access.

When a package function is called as `math.Factor(n)`, the compiler emits
`SetThis` which pushes `math` onto the receiver stack.  After the CALL-5
fix, `getPackageSymbols()` returned the math package's embedded symbol table.
`callBytecodeFunction` then cloned that table and used it as the function's
scope.  The clone only contained the math package's own symbols — not the
global package registry — so any reference inside `Factor` to another package
(e.g. `math.Sqrt`) failed with "unknown identifier".

The root cause: `SetThis` pushes the receiver for ALL member calls, including
plain package-function calls (`math.Floor(x)`) where `callNative` does NOT
pop the receiver (it only calls `popThis()` when `dp.Declaration.Type != nil`).
The receiver stack therefore retains stale package objects, and any subsequent
`*ByteCode` call with a non-empty receiver stack would incorrectly take the
clone path.

The fix: the `getPackageSymbols()` call was removed from `callBytecodeFunction`
entirely.  Compiled Ego functions always use `callFramePush` which creates a
fresh boundary scope as a child of `c.symbols`, giving the function access to
the full scope chain including the global package registry.  The existing
`updatePackageFromLocalSymbols` mechanism in `callFramePop` already handles
writing modified package-level symbols back to the package on return — no
clone path is needed.
