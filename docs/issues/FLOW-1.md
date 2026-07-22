# FLOW-1 — `Test_branchFalseByteCode` called `branchTrueByteCode` for its invalid-address sub-case

**Affected test:** `Test_branchFalseByteCode` in `bytecode/flow_test.go`  
**File:** `bytecode/flow_test.go`  
**Risk:** Low — `branchFalseByteCode`'s address-validation path was not tested
by the legacy test; the gap was covered by `branch_test.go`  
**Discovered by:** code review of `flow_test.go` during the flow-test audit  
**Status: RESOLVED**

## FLOW-1: Original behavior

The third sub-case in `Test_branchFalseByteCode` was intended to verify that
an out-of-range address is rejected:

```go
// Test if target is invalid
_ = ctx.push(true)
e = branchTrueByteCode(ctx, 20)   // ← wrong function: should be branchFalseByteCode
if !e.(*errors.Error).Equal(errors.ErrInvalidBytecodeAddress) {
    t.Errorf("branchFalseByteCode unexpected error %v", e)   // message still says False
}
```

The call targets `branchTrueByteCode` rather than `branchFalseByteCode`.  The
error message in the `t.Errorf` even refers to `branchFalseByteCode`, showing
the intent was clear — but the wrong function was called.  As a result,
`branchFalseByteCode`'s `validateBranchAddress` path for too-large addresses
was never exercised by this test (though it was covered by the later
`Test_branchFalseByteCode_InvalidAddress_TooLarge` in `branch_test.go`).

## FLOW-1: Fix

The call was corrected to target `branchFalseByteCode`:

```go
// Sub-case 3: target address is out of range → ErrInvalidBytecodeAddress.
// FLOW-1 fix: the original code mistakenly called branchTrueByteCode here.
_ = ctx.push(true)
e = branchFalseByteCode(ctx, 20)   // 20 > nextAddress(5) → invalid
if !e.(*errors.Error).Equal(errors.ErrInvalidBytecodeAddress) {
    t.Errorf("branchFalseByteCode sub-case 3 unexpected error %v", e)
}
```
