# CONTEXT-1 — `GetModuleName` panics with a nil pointer dereference when `bc` is nil

**Affected function:** `GetModuleName`  
**File:** `bytecode/context.go`  
**Risk:** Low — any caller that creates a context with a nil `ByteCode` and then
calls `GetModuleName` panics  
**Discovered by:** `Test_Context_GetModuleName_NilBytecode`  
**Status: RESOLVED**

## CONTEXT-1: Description

`GetName` and `GetModuleName` both return the bytecode object's name, but they
differed in nil safety:

```go
// GetName — nil-safe:
func (c *Context) GetName() string {
    if c.bc != nil {
        return c.bc.name
    }
    return defs.Main
}

// GetModuleName — NOT nil-safe (CONTEXT-1 bug):
func (c *Context) GetModuleName() string {
    return c.bc.name   // ← panics when c.bc == nil
}
```

`NewContext` accepts a nil `ByteCode` argument (it sets `c.bc = nil` in that
case).  Any caller that subsequently invoked `GetModuleName` on such a context
received an unrecoverable nil pointer dereference panic.

## CONTEXT-1: Fix

A nil guard identical to the one in `GetName` was added to `GetModuleName`:

```go
func (c *Context) GetModuleName() string {
    if c.bc != nil {
        return c.bc.name
    }
    return defs.Main
}
```

`Test_Context_GetModuleName_NilBytecode` confirms the function returns
`defs.Main` rather than panicking when the context has no bytecode object.
