# EQUAL-1 — `equalTypes` returns an undecorated error (no module or line info)

**Affected function:** `equalTypes`  
**File:** `bytecode/equal.go`  
**Risk:** Low — error messages lack the source location that other runtime errors
include; debuggability is slightly reduced  
**Discovered by:** `Test_equalTypes_TypeVsNonType`  
**Status: RESOLVED**

## EQUAL-1: Original behavior

When `equalTypes` received a v2 value that was neither a `string` nor a
`*data.Type`, it returned an error directly without location annotation:

```go
return errors.ErrNotAType.Context(v2)   // ← no c.runtimeError wrap
```

Every other error path in `equal.go` and the rest of the bytecode package
uses `c.runtimeError(...)`, which annotates the error with the current
module name and source line before returning it.  The direct `return` in
`equalTypes` bypassed that annotation, so an error in a catch block or stack
trace showed only the message key, not where in the Ego program the bad
comparison occurred.

## EQUAL-1: Fix

The return statement was changed to pass the error through `c.runtimeError`,
which attaches the current module name (via `e.In(c.module)`) and source line
(via `e.At(c.GetLine(), 0)`) before returning:

```go
// Before:
return errors.ErrNotAType.Context(v2)

// After:
return c.runtimeError(errors.ErrNotAType, v2)
```

`c.runtimeError` accepts variadic context values and calls `.Context(v2)` on
the annotated error internally, so the v2 value still appears in the message.

`Test_equalTypes_TypeVsNonType_ErrorIsDecorated` confirms that after the fix,
`HasIn()` returns `true` on the error when the context has a non-empty module
name.
