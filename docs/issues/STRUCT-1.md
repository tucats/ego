# STRUCT-1 — Dead code check in `storeInPackage` is unreachable

**Affected function:** `storeInPackage`  
**File:** `bytecode/structs.go`  
**Risk:** None — dead code only; behavior is correct  
**Discovered by:** `Test_storeInPackage_NoDiscardedVariableCheck_STRUCT1`  
**Status: RESOLVED**

## STRUCT-1: Original behavior

`storeInPackage` contained two consecutive guards on the `name` parameter:

```go
// Guard 1: reject unexported (lowercase) names.
if !egostrings.HasCapitalizedName(name) {
    return c.runtimeError(errors.ErrSymbolNotExported, pkg.Name+"."+name)
}

// Guard 2 (dead code): reject names starting with "_".
if name[0:1] == defs.DiscardedVariable {
    return c.runtimeError(errors.ErrReadOnlyValue, pkg.Name+"."+name)
}
```

`egostrings.HasCapitalizedName` returns `true` only when the first Unicode
character of the name is uppercase (A–Z or Unicode uppercase).  The underscore
character `_` is **not** uppercase, so any name starting with `_` returned
`false` from guard 1 and exited with `ErrSymbolNotExported` before reaching
guard 2.  Guard 2 could therefore never execute.

`defs.DiscardedVariable = "_"`, so the intent of guard 2 was probably to
catch names like `"_internalHelper"` — but such names were already caught by
guard 1.

## STRUCT-1: Fix

Guard 2 (the unreachable `name[0:1] == defs.DiscardedVariable` block) was
removed entirely.  The function now has a single exported-name check followed
directly by the read-only and constant checks.

```go
// Must be an exported (capitalized) name.
if !egostrings.HasCapitalizedName(name) {
    return c.runtimeError(errors.ErrSymbolNotExported, pkg.Name+"."+name)
}

// If it's a declared item in the package, is it one of the ones
// that is readOnly by default?
if oldItem, found := pkg.Get(name); found { ... }
```

`Test_storeInPackage_NoDiscardedVariableCheck_STRUCT1` confirms that a name
like `"_Foo"` is still correctly rejected by the first guard with
`ErrSymbolNotExported`, and that no `defs` import is required in `structs.go`
solely for this guard.
