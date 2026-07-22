# CALL-3 — Nil pointer dereference in callRuntimeFunction when savedDefinition is nil and context is sandboxed

**Affected function:** `callRuntimeFunction`  
**File:** `bytecode/callRuntimeFunction.go`  
**Risk:** High in sandboxed deployments — accessing `savedDefinition.Sandboxed`
panics when `savedDefinition == nil`  
**Discovered by:** code review during call.go test development  
**Status: RESOLVED**

## CALL-3: Original behavior

`callRuntimeFunction` included this guard:

```go
if c.sandboxedIO.Load() && savedDefinition.Sandboxed {
    return errors.ErrNoPrivilegeForOperation...
}
```

When a bare `func(*symbols.SymbolTable, data.List) (any, error)` was pushed
directly onto the stack (not wrapped in a `data.Function`), `callByteCode`
passed `savedDefinition = nil` to `callRuntimeFunction`.  If the execution
context was sandboxed (`c.sandboxedIO.Load()` returns `true`), Go's `&&`
operator did not short-circuit because the left operand was true, and
`savedDefinition.Sandboxed` panicked with a nil pointer dereference.

In the non-sandboxed default the panic did not occur because the left operand
was false.  The bug was latent in production server code that uses
`ctx.Sandboxed(true)`.

## CALL-3: Fix

A nil check was added as the middle operand:

```go
if c.sandboxedIO.Load() && savedDefinition != nil && savedDefinition.Sandboxed {
    return errors.ErrNoPrivilegeForOperation.Context(savedDefinition.Declaration.Name + "()")
}
```

When `savedDefinition` is nil the middle operand short-circuits and the field
access is never reached.  The function proceeds normally — the bare runtime
function is not sandboxed, so no privilege check is needed.

`Test_callRuntimeFunction_NilDefinition_SandboxedNoPanic` confirms that calling
a bare runtime function with `sandboxedIO = true` no longer panics.
