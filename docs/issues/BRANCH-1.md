# BRANCH-1 — Stack mutated before address validation in conditional branches

**Affected instructions:** `branchFalseByteCode`, `branchTrueByteCode`  
**File:** `bytecode/branch.go`  
**Risk:** Medium — stack corruption visible after caught runtime errors  
**Discovered by:** `Test_branchFalseByteCode_StackPreservedOnAddressError`,
`Test_branchTrueByteCode_StackPreservedOnAddressError`  
**Status: RESOLVED**

## BRANCH-1: Original behavior

Both conditional branch instructions popped the top-of-stack (TOS) value
**before** validating the branch destination address.  The sequence was:

```text
1. v, err := c.Pop()         ← TOS consumed here
2. [strict-mode type check]
3. if address out-of-range → return ErrInvalidBytecodeAddress
```

When step 3 returned an error the Pop in step 1 had already been committed.
If the error was caught by a surrounding `try/catch` block in Ego code, the
stack was one item shorter than the caller expected, causing subsequent
instructions to read wrong values or underflow.

## BRANCH-1: Fix

The `validateBranchAddress` helper was extracted and called **before** any
`c.Pop()` call.  The new execution order is:

1. Validate address (no stack side effects).
2. `c.Pop()` — only reached when the address is known valid.
3. Strict-mode type constraint.
4. Coerce to bool and conditionally update the program counter.

Tests `Test_branchFalseByteCode_StackPreservedOnAddressError` and
`Test_branchTrueByteCode_StackPreservedOnAddressError` confirm that the TOS
value remains on the stack after an address-validation error.
