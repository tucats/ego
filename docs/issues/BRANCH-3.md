# BRANCH-3 — Misleading error context for non-integer operands

**Affected instructions:** `branchByteCode`, `branchFalseByteCode`,
`branchTrueByteCode`  
**File:** `bytecode/branch.go`  
**Risk:** Low — only affects diagnostic error messages, not correctness  
**Discovered by:** `Test_branchByteCode_NonIntOperand`  
**Status: RESOLVED**

## BRANCH-3: Original behavior

When `data.Int(i)` failed because the operand was not an integer, the code
used the `address` variable (zero-valued on failure) as the error context:

```go
if address, err := data.Int(i); err != nil || ... {
    return c.runtimeError(errors.ErrInvalidBytecodeAddress).Context(address)
}
```

The error message read `"invalid bytecode address: 0"` even when the actual
operand was, for example, the string `"not-a-number"`.

## BRANCH-3: Fix

The `validateBranchAddress` helper separates the two failure modes.  When
`data.Int` itself fails the original operand `i` is used as context; when the
conversion succeeds but the address is out of range the numeric address is
used:

```go
address, err := data.Int(i)
if err != nil {
    // Show the actual bad operand, not the zero-value placeholder.
    return 0, c.runtimeError(errors.ErrInvalidBytecodeAddress).Context(i)
}
if address < 0 || address > c.bc.nextAddress {
    // Address is a valid int but out of the legal range.
    return 0, c.runtimeError(errors.ErrInvalidBytecodeAddress).Context(address)
}
```
