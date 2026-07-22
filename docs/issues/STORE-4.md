# STORE-4 — `storeChanByteCode` passes nil (`x`) as error context instead of the variable name

**Affected function:** `storeChanByteCode`  
**File:** `bytecode/store.go`  
**Risk:** Low — the error is still returned with the correct error key
(`ErrUnknownIdentifier`); only the context string in the error message is wrong  
**Discovered by:** `Test_storeChanByteCode_NonChanDestVarNotFound`  
**Status: RESOLVED**

## STORE-4: Original behavior

When the stack value is not a channel and the destination variable does not
exist, `storeChanByteCode` built the error using `x` (the un-found value, which
is always `nil`):

```go
x, found := c.get(variableName)
if !found {
    if sourceChan {
        err = c.create(variableName)
    } else {
        err = c.runtimeError(errors.ErrUnknownIdentifier).Context(x)  // x is nil
    }
}
```

The error message read `"unknown identifier: <nil>"` instead of
`"unknown identifier: missing"`.

## STORE-4: Fix

`.Context(x)` was replaced with `.Context(variableName)`:

```go
err = c.runtimeError(errors.ErrUnknownIdentifier).Context(variableName)
```

`Test_storeChanByteCode_NonChanDestVarNotFound` was strengthened to assert
both the error key and that the variable name `"missing"` appears in the
error message via `strings.Contains`.
