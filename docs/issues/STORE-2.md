# STORE-2 — `defs.DiscardedVariable` used where `defs.ReadonlyVariablePrefix` is intended

**Affected functions:** `storeByteCode`, `storeGlobalByteCode`, `storeAlwaysByteCode`  
**File:** `bytecode/store.go`  
**Risk:** None — both constants equal `"_"` so behavior is identical; but the
wrong constant name obscures intent and could cause confusion if either constant
is ever changed to a different value  
**Discovered by:** comment review during `store_test.go` development  
**Status: RESOLVED**

## STORE-2: Original behavior

Three guards in `store.go` checked whether a variable name started with the
readonly prefix by comparing against `defs.DiscardedVariable`:

```go
// storeByteCode (line ~97 before fix):
if strings.HasPrefix(name, defs.DiscardedVariable) { ... }

// storeGlobalByteCode (line ~185 before fix):
if len(name) > 1 && name[0:1] == defs.DiscardedVariable { ... }

// storeAlwaysByteCode (line ~503 before fix):
if len(symbolName) > 1 && symbolName[0:1] == defs.DiscardedVariable { ... }
```

`defs.DiscardedVariable = "_"` is the blank identifier used to discard values
(as in `_ = someExpr`).  The guards are checking for the **readonly prefix**,
which is `defs.ReadonlyVariablePrefix = "_"`.  Both constants happen to be
`"_"`, so the behavior is correct today — but using the wrong constant
communicates the wrong intent.

## STORE-2: Fix

All three guards were updated to use `defs.ReadonlyVariablePrefix`:

```go
// storeByteCode:
if strings.HasPrefix(name, defs.ReadonlyVariablePrefix) { ... }

// storeGlobalByteCode:
if len(name) > 1 && name[0:1] == defs.ReadonlyVariablePrefix { ... }

// storeAlwaysByteCode:
if len(symbolName) > 1 && symbolName[0:1] == defs.ReadonlyVariablePrefix { ... }
```
