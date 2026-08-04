# STORE-1 — Misleading comment in `storeByteCode` for the readonly-prefix branch

**Affected function:** `storeByteCode`  
**File:** `bytecode/store.go`  
**Risk:** None — documentation only; behavior was correct  
**Discovered by:** comment review during `store_test.go` development  
**Status: RESOLVED**

## STORE-1: Original behavior

The comment immediately before the `strings.HasPrefix` guard read:

```go
// If we are writing to the "_" variable, no action is taken.
if strings.HasPrefix(name, defs.DiscardedVariable) {
    return c.set(name, data.Constant(value))
}
```

This was wrong on two counts:

1. The `name == "_"` (exact discard) case had already been handled and returned
   `nil` four lines earlier.  The `HasPrefix` guard handles all OTHER names that
   start with `"_"` (e.g., `"_foo"`, `"_bar"`).
2. "No action is taken" is the opposite of the actual behavior: the function
   **does** store the value, wrapping it in `data.Constant` to make it immutable.

## STORE-1: Fix

The comment was rewritten to accurately describe both cases:

```go
// Variables whose names start with "_" (the readonly prefix) receive
// their value wrapped in data.Constant so that subsequent loads see
// an immutable value.  The readonly-existence check above already
// ensured the variable exists and holds symbols.UndefinedValue, so
// this is always the first (and only) write to the variable.
if strings.HasPrefix(name, defs.ReadonlyVariablePrefix) {
    return c.set(name, data.Constant(value))
}
```
