# STACK-3 — `dropByteCode` silently swallows stack underflow when dropping more items than exist

**Affected function:** `dropByteCode`  
**File:** `bytecode/stack.go`  
**Risk:** Low — over-drops are silently accepted; callers that rely on Drop
removing a precise number of items cannot detect that fewer items were actually
removed  
**Discovered by:** `Test_dropByteCode_SilentUnderflow_STACK3`  
**Status: RESOLVED**

## STACK-3: Description

When the stack ran dry before `count` items had been dropped, `dropByteCode`
returned `nil` instead of propagating the underflow error, inconsistently with
every other stack-consuming function in the package.

## STACK-3: Fix

Implemented Option A (strict): `return nil` changed to `return err` inside the
Pop loop, with an explanatory comment:

```go
for n := 0; n < count; n++ {
    if _, err = c.Pop(); err != nil {
        // Propagate stack underflow rather than silently swallowing it
        // (STACK-3 fix).
        return err
    }
}
```

The 869-test Ego integration suite confirmed that no existing program relies on
the silent over-drop behavior.

Test renamed from `_SilentUnderflow_STACK3` to `_UnderflowReturnsError_STACK3`;
now asserts `ErrStackUnderflow` rather than logging that nil was returned.
