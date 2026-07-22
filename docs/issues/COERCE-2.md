# COERCE-2 — `data.UInt` accessor panics with a type assertion failure

**Affected path:** `coerceByteCode` → `data.UInt`  
**File:** `data/coerce.go` (root cause); `data/accessor.go` (assertion site)  
**Risk:** Medium — any Ego program that coerced a value to the `uint` type
panicked instead of producing a clean runtime result  
**Discovered by:** `Test_coerceByteCode_ToUInt`  
**Status: RESOLVED**

## COERCE-2: Original behavior

The package-level `Coerce(value, model)` function routed `case uint:` to
`coerceUInt64`, which returns `uint64`.  `data.UInt()` then asserted the
result as `uint`, causing a panic:

```text
panic: interface conversion: interface {} is uint64, not uint
```

## COERCE-2: Fix

A dedicated `coerceUInt` helper was added to `data/coerce.go`.  It mirrors
`coerceUInt64` in structure but returns `uint` for every input type.  The
`case uint:` branch of the package-level `Coerce` function now calls
`coerceUInt` instead of `coerceUInt64`, so `data.UInt()` receives the correct
concrete type and the type assertion succeeds.

`Test_coerceByteCode_ToUInt` now asserts a clean `nil` error and `uint(10)`
on the stack rather than catching a panic.
