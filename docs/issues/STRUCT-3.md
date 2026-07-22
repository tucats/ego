# STRUCT-3 — `flattenByteCode` produces incorrect `argCountDelta` for empty arrays

**Affected function:** `flattenByteCode`  
**File:** `bytecode/structs.go`  
**Risk:** Low — the Ego compiler does not appear to spread empty arrays at
call sites; the bug was latent and required a malformed or hand-crafted
bytecode sequence to trigger  
**Discovered by:** `Test_flattenByteCode_EmptyArray_STRUCT3`  
**Status: RESOLVED**

## STRUCT-3: Original behavior

`flattenByteCode` replaced the top-of-stack array with its individual elements
and set `c.argCountDelta` to `N-1`, where `N` is the number of elements
expanded.  The "-1" accounted for the fact that the compiler already counted the
original array as one argument.

The decrement was guarded by `argCountDelta > 0`:

```go
// After the expansion loop:
if c.argCountDelta > 0 {
    c.argCountDelta--   // net: N-1 for N > 0, but 0 for N = 0 (wrong)
}
```

For an **empty** array (`N == 0`), the loop did not execute, `argCountDelta`
stayed at 0, and the guard prevented the decrement.  The following `Call`
opcode computed `argc = compiler_count + argCountDelta = 1 + 0 = 1`, but the
stack had 0 expanded values — causing the call to either underflow the stack
or pop a stale value as an argument.

## STRUCT-3: Fix

The `if c.argCountDelta > 0` conditional was replaced with an `isArray` flag
that tracks whether a `*data.Array` or `[]any` was expanded.  The decrement
now runs unconditionally for any array expansion, including empty arrays:

```go
isArray := false

if array, ok := v.(*data.Array); ok {
    isArray = true
    for idx := 0; idx < array.Len(); idx++ { ... c.argCountDelta++ }
} else if array, ok := v.([]any); ok {
    isArray = true
    for _, vv := range array { ... c.argCountDelta++ }
} else {
    _ = c.push(v)   // scalar: no delta adjustment
}

// Subtract 1 for any array, including empty, to remove the original slot.
if isArray {
    c.argCountDelta--
}
```

Result for each case:

| Input | Elements pushed | argCountDelta |
| :---- | :-------------- | :------------ |
| empty array (N=0) | 0 | 0 − 1 = **−1** ✓ |
| single-element array (N=1) | 1 | 1 − 1 = 0 ✓ |
| three-element array (N=3) | 3 | 3 − 1 = 2 ✓ |
| scalar (not an array) | 1 (unchanged) | 0 ✓ |

`Test_flattenByteCode_EmptyArray_STRUCT3` now asserts `argCountDelta == -1`
after flattening an empty array, confirming correct behavior.  The full
869-test Ego integration suite continued to pass after the change.
