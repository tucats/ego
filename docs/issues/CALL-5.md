# CALL-5 — `getPackageSymbols` passes the `this` struct instead of `this.value` to `GetPackageSymbolTable`

**Affected function:** `getPackageSymbols`  
**File:** `bytecode/flow.go`  
**Risk:** Medium — the package-method cloning path in `callBytecodeFunction`
is permanently unreachable; package method calls silently fall back to the
generic `callFramePush` path  
**Discovered by:** `Test_getPackageSymbols_PackageReceiver_ReturnsSymbolTable`  
**Status: RESOLVED**

## CALL-5: Original behavior

`getPackageSymbols` passed the raw `this` struct (the bookkeeping wrapper) to
`GetPackageSymbolTable` instead of the unwrapped `receiver.value` field:

```go
this := c.receiverStack[len(c.receiverStack)-1]
table := symbols.GetPackageSymbolTable(this)   // ← was: this (the wrapper struct)
```

`c.receiverStack` stores `this{name string, value any}` structs.
`GetPackageSymbolTable` type-asserts its argument to `*data.Package`.  The
assertion on a `this` struct always failed, `nil` was returned, and
`callBytecodeFunction` always took the plain `callFramePush` branch.  The
package-method clone path was permanently dead.

## CALL-5: Fix

One word changed in `flow.go`: the `this` local variable was renamed to
`receiver` (for clarity) and `receiver.value` is now passed:

```go
receiver := c.receiverStack[len(c.receiverStack)-1]
table := symbols.GetPackageSymbolTable(receiver.value)
```

The existing tests were updated:

- `Test_getPackageSymbols_PackageReceiver_CurrentlyReturnsNil` was renamed to
  `Test_getPackageSymbols_PackageReceiver_ReturnsSymbolTable` and now asserts
  a non-nil result with the expected table name `"package math"`.
- `Test_callBytecodeFunction_PackageMethod_ClonesPackageSymbolTable` was added
  to verify the previously dead clone path is now reachable and that the
  resulting scope carries `IsClone() == true`.
