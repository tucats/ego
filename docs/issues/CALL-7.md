# CALL-7 — `callTypeCast` panics when Path-A struct types receive empty argument list

**Affected function:** `callTypeCast`  
**File:** `bytecode/callCastFunction.go`  
**Risk:** Medium — a well-formed Ego program that calls a struct-based type
constructor with no arguments (e.g. `time.Duration()`) caused an unrecoverable
runtime panic instead of a clean error  
**Discovered by:** `Test_callTypeCast_Duration_EmptyArgs`,
`Test_callTypeCast_Month_EmptyArgs`  
**Status: RESOLVED**

## CALL-7: Original behavior

`callTypeCast` dispatched on the kind of the target type.  When the type was a
`StructKind` or a `TypeKind` wrapping a `StructKind` (Path A), the function
accessed `args[0]` directly without first checking that the slice was non-empty:

```go
case defs.TimeDurationTypeName:
    if d, err := data.Int64(args[0]); err == nil {  // ← panic if args is empty
```

When `callByteCode` called `callTypeCast` with `argc == 0` (the user wrote
`time.Duration()` or `time.Month()` with no arguments), `args` was an empty
slice and either access panicked with a runtime index-out-of-bounds error.

Path B (scalar/array types) was not affected because it appends the type to
`args` and delegates to `builtins.Cast`, which handles empty lists gracefully.

## CALL-7: Fix

A single bounds check was added at the top of the Path-A block, before the
switch statement:

```go
if function.Kind() == data.StructKind || ... {
    // Guard against zero-argument calls (CALL-7 fix).
    // Struct-based type constructors always require exactly one argument.
    if len(args) == 0 {
        return c.runtimeError(errors.ErrArgumentCount)
    }
    switch function.NativeName() {
    ...
    }
}
```

`Test_callTypeCast_Duration_EmptyArgs` and `Test_callTypeCast_Month_EmptyArgs`
now assert `ErrArgumentCount` and an empty stack rather than catching a panic.
