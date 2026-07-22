# BRANCH-2 — nil operand silently accepted as address 0

**Affected instructions:** `branchByteCode`, `branchFalseByteCode`,
`branchTrueByteCode`  
**File:** `bytecode/branch.go`  
**Risk:** Low — only triggers if the compiler emits a nil operand, which is
not expected in well-formed bytecode  
**Discovered by:** `Test_branchByteCode_NilOperand`,
`Test_branchFalseByteCode_NilOperand_Rejected`,
`Test_branchTrueByteCode_NilOperand_Rejected`  
**Status: RESOLVED**

## BRANCH-2: Original behavior

`data.Int(nil)` returns `(0, nil)`.  Since address 0 is always a valid branch
target, passing `nil` as the operand to any branch instruction silently set the
program counter to 0 rather than returning an error.  This masked compiler bugs
where a branch operand was accidentally left nil.

## BRANCH-2: Fix

An explicit nil check was added at the top of `validateBranchAddress` (called
by all three branch instructions) before `data.Int` is invoked:

```go
if i == nil {
    return 0, c.runtimeError(errors.ErrInvalidBytecodeAddress).Context("nil")
}
```

`Test_branchByteCode_NilOperand` was updated to expect `ErrInvalidBytecodeAddress`
instead of `nil`.  Two new tests were added for the conditional variants.
