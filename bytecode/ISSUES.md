# Bytecode Instruction Issues

This file documents behavioral anomalies, potential bugs, and design concerns
found during the comprehensive bytecode unit-test effort.  Each entry includes
the affected instruction(s), a description of the original behavior, the risk
level, and the resolution.

Entries are added as tests discover them — the tests themselves contain
`// See bytecode/ISSUES.md` comments pointing here.

---

## BRANCH-1 — Stack mutated before address validation in conditional branches

**Affected instructions:** `branchFalseByteCode`, `branchTrueByteCode`  
**File:** `bytecode/branch.go`  
**Risk:** Medium — stack corruption visible after caught runtime errors  
**Discovered by:** `Test_branchFalseByteCode_StackPreservedOnAddressError`,
`Test_branchTrueByteCode_StackPreservedOnAddressError`  
**Status: RESOLVED**

### BRANCH-1: Original behavior

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

### BRANCH-1: Fix

The `validateBranchAddress` helper was extracted and called **before** any
`c.Pop()` call.  The new execution order is:

1. Validate address (no stack side effects).
2. `c.Pop()` — only reached when the address is known valid.
3. Strict-mode type constraint.
4. Coerce to bool and conditionally update the program counter.

Tests `Test_branchFalseByteCode_StackPreservedOnAddressError` and
`Test_branchTrueByteCode_StackPreservedOnAddressError` confirm that the TOS
value remains on the stack after an address-validation error.

---

## BRANCH-2 — nil operand silently accepted as address 0

**Affected instructions:** `branchByteCode`, `branchFalseByteCode`,
`branchTrueByteCode`  
**File:** `bytecode/branch.go`  
**Risk:** Low — only triggers if the compiler emits a nil operand, which is
not expected in well-formed bytecode  
**Discovered by:** `Test_branchByteCode_NilOperand`,
`Test_branchFalseByteCode_NilOperand_Rejected`,
`Test_branchTrueByteCode_NilOperand_Rejected`  
**Status: RESOLVED**

### BRANCH-2: Original behavior

`data.Int(nil)` returns `(0, nil)`.  Since address 0 is always a valid branch
target, passing `nil` as the operand to any branch instruction silently set the
program counter to 0 rather than returning an error.  This masked compiler bugs
where a branch operand was accidentally left nil.

### BRANCH-2: Fix

An explicit nil check was added at the top of `validateBranchAddress` (called
by all three branch instructions) before `data.Int` is invoked:

```go
if i == nil {
    return 0, c.runtimeError(errors.ErrInvalidBytecodeAddress).Context("nil")
}
```

`Test_branchByteCode_NilOperand` was updated to expect `ErrInvalidBytecodeAddress`
instead of `nil`.  Two new tests were added for the conditional variants.

---

## BRANCH-3 — Misleading error context for non-integer operands

**Affected instructions:** `branchByteCode`, `branchFalseByteCode`,
`branchTrueByteCode`  
**File:** `bytecode/branch.go`  
**Risk:** Low — only affects diagnostic error messages, not correctness  
**Discovered by:** `Test_branchByteCode_NonIntOperand`  
**Status: RESOLVED**

### BRANCH-3: Original behavior

When `data.Int(i)` failed because the operand was not an integer, the code
used the `address` variable (zero-valued on failure) as the error context:

```go
if address, err := data.Int(i); err != nil || ... {
    return c.runtimeError(errors.ErrInvalidBytecodeAddress).Context(address)
}
```

The error message read `"invalid bytecode address: 0"` even when the actual
operand was, for example, the string `"not-a-number"`.

### BRANCH-3: Fix

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
