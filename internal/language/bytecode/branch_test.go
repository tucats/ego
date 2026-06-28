package bytecode

// branch_test.go contains unit tests for the three branch bytecode handlers
// in branch.go:
//
//   - branchByteCode        – unconditional branch to address
//   - branchFalseByteCode   – branch to address if TOS is falsy
//   - branchTrueByteCode    – branch to address if TOS is truthy
//
// # How addresses work
//
// Every branch instruction validates its operand against c.bc.nextAddress.
// An address is valid when 0 ≤ address ≤ nextAddress.  A freshly created
// ByteCode has nextAddress == 0, so the only valid address is 0.  Tests that
// need to branch to a non-zero address first call tc.withBytecodeSize(n) to
// widen the window.
//
// # Program counter assertions
//
// When calling a bytecode function directly (not through Run), the program
// counter starts at 0.  After a taken branch it equals the target address;
// after a not-taken branch it remains 0.  Use tc.assertProgramCounter to
// verify either outcome.
//
// # Sections
//
//  1. branchByteCode – unconditional
//  2. branchFalseByteCode – conditional on false
//  3. branchTrueByteCode – conditional on true
//  4. Shared address-validation behaviour (cross-instruction)

import (
	"testing"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// ─── Section 1: branchByteCode (unconditional) ───────────────────────────────

// Test_branchByteCode_ToValidAddress verifies that a valid mid-range address
// sets the program counter to that address.
func Test_branchByteCode_ToValidAddress(t *testing.T) {
	tc := newTestContext(t).withBytecodeSize(10)

	err := branchByteCode(tc.ctx, 7)

	tc.assertNoError(err)
	tc.assertProgramCounter(7)
}

// Test_branchByteCode_ToAddressZero verifies that branching to address 0 is
// always valid (nextAddress starts at 0, and 0 ≤ 0 ≤ 0).
func Test_branchByteCode_ToAddressZero(t *testing.T) {
	tc := newTestContext(t) // nextAddress stays 0

	err := branchByteCode(tc.ctx, 0)

	tc.assertNoError(err)
	tc.assertProgramCounter(0)
}

// Test_branchByteCode_ToAddressEqualNextAddress verifies the upper boundary:
// an address exactly equal to nextAddress is valid.
func Test_branchByteCode_ToAddressEqualNextAddress(t *testing.T) {
	tc := newTestContext(t).withBytecodeSize(5)

	err := branchByteCode(tc.ctx, 5) // == nextAddress

	tc.assertNoError(err)
	tc.assertProgramCounter(5)
}

// Test_branchByteCode_NegativeAddress verifies that a negative operand is
// rejected with ErrInvalidBytecodeAddress.
func Test_branchByteCode_NegativeAddress(t *testing.T) {
	tc := newTestContext(t).withBytecodeSize(10)

	err := branchByteCode(tc.ctx, -1)

	tc.assertError(err, errors.ErrInvalidBytecodeAddress)
}

// Test_branchByteCode_AddressExceedsNextAddress verifies that an address one
// past nextAddress is rejected.
func Test_branchByteCode_AddressExceedsNextAddress(t *testing.T) {
	tc := newTestContext(t).withBytecodeSize(5)

	err := branchByteCode(tc.ctx, 6) // 6 > nextAddress(5)

	tc.assertError(err, errors.ErrInvalidBytecodeAddress)
}

// Test_branchByteCode_NonIntOperand verifies that an operand that cannot be
// converted to an integer by data.Int (e.g. a plain string) is rejected.
func Test_branchByteCode_NonIntOperand(t *testing.T) {
	tc := newTestContext(t).withBytecodeSize(10)

	err := branchByteCode(tc.ctx, "not-a-number")

	tc.assertError(err, errors.ErrInvalidBytecodeAddress)
}

// Test_branchByteCode_NilOperand verifies that a nil operand is now rejected
// with ErrInvalidBytecodeAddress.  Prior to the BRANCH-2 fix, data.Int(nil)
// returned (0, nil), causing nil to be silently accepted as "branch to address 0".
func Test_branchByteCode_NilOperand(t *testing.T) {
	tc := newTestContext(t)

	err := branchByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrInvalidBytecodeAddress)
}

// Test_branchFalseByteCode_NilOperand_Rejected verifies that the BRANCH-2 fix
// applies to branchFalseByteCode as well: a nil operand is rejected rather
// than silently treated as address 0.
func Test_branchFalseByteCode_NilOperand_Rejected(t *testing.T) {
	tc := newTestContext(t).withStack(false)

	err := branchFalseByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrInvalidBytecodeAddress)
}

// Test_branchTrueByteCode_NilOperand_Rejected verifies that the BRANCH-2 fix
// applies to branchTrueByteCode as well.
func Test_branchTrueByteCode_NilOperand_Rejected(t *testing.T) {
	tc := newTestContext(t).withStack(true)

	err := branchTrueByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrInvalidBytecodeAddress)
}

// Test_branchByteCode_IntStringOperand verifies that a string whose content
// is a valid integer is accepted (data.Int coerces "3" → 3).
func Test_branchByteCode_IntStringOperand(t *testing.T) {
	tc := newTestContext(t).withBytecodeSize(10)

	err := branchByteCode(tc.ctx, "3")

	tc.assertNoError(err)
	tc.assertProgramCounter(3)
}

// Test_branchByteCode_LeavesStackUntouched verifies that the unconditional
// branch does not consume any stack items.
func Test_branchByteCode_LeavesStackUntouched(t *testing.T) {
	tc := newTestContext(t).
		withBytecodeSize(5).
		withStack("sentinel")

	err := branchByteCode(tc.ctx, 2)

	tc.assertNoError(err)
	tc.assertTopStack("sentinel") // still present
}

// ─── Section 2: branchFalseByteCode ──────────────────────────────────────────

// Test_branchFalseByteCode_TOS_False_BranchTaken verifies that when TOS is
// the boolean false the program counter is set to the target address.
func Test_branchFalseByteCode_TOS_False_BranchTaken(t *testing.T) {
	tc := newTestContext(t).
		withBytecodeSize(10).
		withStack(false)

	err := branchFalseByteCode(tc.ctx, 6)

	tc.assertNoError(err)
	tc.assertProgramCounter(6)
}

// Test_branchFalseByteCode_TOS_True_NoBranch verifies that when TOS is true
// the program counter is left unchanged (branch not taken).
func Test_branchFalseByteCode_TOS_True_NoBranch(t *testing.T) {
	tc := newTestContext(t).
		withBytecodeSize(10).
		withStack(true)

	err := branchFalseByteCode(tc.ctx, 6)

	tc.assertNoError(err)
	tc.assertProgramCounter(0) // PC unchanged
}

// Test_branchFalseByteCode_IntZero_BranchTaken verifies that integer 0 coerces
// to false in non-strict mode and causes a branch.
func Test_branchFalseByteCode_IntZero_BranchTaken(t *testing.T) {
	tc := newTestContext(t).
		withBytecodeSize(10).
		withTypeStrictness(defs.NoTypeEnforcement).
		withStack(0)

	err := branchFalseByteCode(tc.ctx, 4)

	tc.assertNoError(err)
	tc.assertProgramCounter(4)
}

// Test_branchFalseByteCode_IntNonZero_NoBranch verifies that a non-zero integer
// coerces to true in non-strict mode and suppresses the branch.
func Test_branchFalseByteCode_IntNonZero_NoBranch(t *testing.T) {
	tc := newTestContext(t).
		withBytecodeSize(10).
		withTypeStrictness(defs.NoTypeEnforcement).
		withStack(42)

	err := branchFalseByteCode(tc.ctx, 4)

	tc.assertNoError(err)
	tc.assertProgramCounter(0)
}

// Test_branchFalseByteCode_StringFalse_BranchTaken verifies that the string
// "false" coerces to the boolean false in non-strict mode.
func Test_branchFalseByteCode_StringFalse_BranchTaken(t *testing.T) {
	tc := newTestContext(t).
		withBytecodeSize(10).
		withTypeStrictness(defs.NoTypeEnforcement).
		withStack("false")

	err := branchFalseByteCode(tc.ctx, 3)

	tc.assertNoError(err)
	tc.assertProgramCounter(3)
}

// Test_branchFalseByteCode_StringTrue_NoBranch verifies that the string "true"
// coerces to the boolean true and suppresses the branch.
func Test_branchFalseByteCode_StringTrue_NoBranch(t *testing.T) {
	tc := newTestContext(t).
		withBytecodeSize(10).
		withTypeStrictness(defs.NoTypeEnforcement).
		withStack("true")

	err := branchFalseByteCode(tc.ctx, 3)

	tc.assertNoError(err)
	tc.assertProgramCounter(0)
}

// Test_branchFalseByteCode_StackUnderflow verifies that an empty stack returns
// ErrStackUnderflow before any address validation occurs.
func Test_branchFalseByteCode_StackUnderflow(t *testing.T) {
	tc := newTestContext(t).withBytecodeSize(10)
	// no withStack call

	err := branchFalseByteCode(tc.ctx, 5)

	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_branchFalseByteCode_StrictMode_NonBool_Rejected verifies that in strict
// mode a non-boolean TOS value is rejected with ErrConditionalBool.
func Test_branchFalseByteCode_StrictMode_NonBool_Rejected(t *testing.T) {
	tc := newTestContext(t).
		withBytecodeSize(10).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withStack(0) // integer — valid in non-strict, rejected in strict

	err := branchFalseByteCode(tc.ctx, 5)

	tc.assertError(err, errors.ErrConditionalBool)
}

// Test_branchFalseByteCode_StrictMode_Bool_Accepted verifies that in strict
// mode a genuine boolean value is accepted.
func Test_branchFalseByteCode_StrictMode_Bool_Accepted(t *testing.T) {
	tc := newTestContext(t).
		withBytecodeSize(10).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withStack(false)

	err := branchFalseByteCode(tc.ctx, 5)

	tc.assertNoError(err)
	tc.assertProgramCounter(5) // false → branch taken
}

// Test_branchFalseByteCode_InvalidAddress_Negative verifies that a negative
// branch address is rejected even when the condition would take the branch.
func Test_branchFalseByteCode_InvalidAddress_Negative(t *testing.T) {
	tc := newTestContext(t).
		withBytecodeSize(10).
		withStack(false)

	err := branchFalseByteCode(tc.ctx, -1)

	tc.assertError(err, errors.ErrInvalidBytecodeAddress)
}

// Test_branchFalseByteCode_InvalidAddress_TooLarge verifies that an address
// beyond nextAddress is rejected.
func Test_branchFalseByteCode_InvalidAddress_TooLarge(t *testing.T) {
	tc := newTestContext(t).
		withBytecodeSize(5).
		withStack(false)

	err := branchFalseByteCode(tc.ctx, 99)

	tc.assertError(err, errors.ErrInvalidBytecodeAddress)
}

// Test_branchFalseByteCode_StackPreservedOnAddressError verifies that after
// the BRANCH-1 fix the address is validated BEFORE the TOS is popped.  When
// the address is invalid the condition value must remain on the stack so that
// any enclosing try/catch block sees an unmodified stack.
func Test_branchFalseByteCode_StackPreservedOnAddressError(t *testing.T) {
	tc := newTestContext(t).
		withBytecodeSize(5).
		withStack(false)

	// Invalid address — the instruction errors before it ever pops TOS.
	_ = branchFalseByteCode(tc.ctx, 99)

	// The condition value must still be on the stack.
	tc.assertTopStack(false)
}

// ─── Section 3: branchTrueByteCode ───────────────────────────────────────────

// Test_branchTrueByteCode_TOS_True_BranchTaken verifies that TOS=true causes
// the program counter to be updated.
func Test_branchTrueByteCode_TOS_True_BranchTaken(t *testing.T) {
	tc := newTestContext(t).
		withBytecodeSize(10).
		withStack(true)

	err := branchTrueByteCode(tc.ctx, 8)

	tc.assertNoError(err)
	tc.assertProgramCounter(8)
}

// Test_branchTrueByteCode_TOS_False_NoBranch verifies that TOS=false leaves
// the program counter unchanged.
func Test_branchTrueByteCode_TOS_False_NoBranch(t *testing.T) {
	tc := newTestContext(t).
		withBytecodeSize(10).
		withStack(false)

	err := branchTrueByteCode(tc.ctx, 8)

	tc.assertNoError(err)
	tc.assertProgramCounter(0)
}

// Test_branchTrueByteCode_IntNonZero_BranchTaken verifies that a non-zero
// integer coerces to true in non-strict mode and causes a branch.
func Test_branchTrueByteCode_IntNonZero_BranchTaken(t *testing.T) {
	tc := newTestContext(t).
		withBytecodeSize(10).
		withTypeStrictness(defs.NoTypeEnforcement).
		withStack(1)

	err := branchTrueByteCode(tc.ctx, 9)

	tc.assertNoError(err)
	tc.assertProgramCounter(9)
}

// Test_branchTrueByteCode_IntZero_NoBranch verifies that integer 0 coerces to
// false in non-strict mode and suppresses the branch.
func Test_branchTrueByteCode_IntZero_NoBranch(t *testing.T) {
	tc := newTestContext(t).
		withBytecodeSize(10).
		withTypeStrictness(defs.NoTypeEnforcement).
		withStack(0)

	err := branchTrueByteCode(tc.ctx, 9)

	tc.assertNoError(err)
	tc.assertProgramCounter(0)
}

// Test_branchTrueByteCode_StringTrue_BranchTaken verifies that the string
// "true" coerces to true in non-strict mode.
func Test_branchTrueByteCode_StringTrue_BranchTaken(t *testing.T) {
	tc := newTestContext(t).
		withBytecodeSize(10).
		withTypeStrictness(defs.NoTypeEnforcement).
		withStack("true")

	err := branchTrueByteCode(tc.ctx, 2)

	tc.assertNoError(err)
	tc.assertProgramCounter(2)
}

// Test_branchTrueByteCode_StringFalse_NoBranch verifies that the string
// "false" coerces to false and suppresses the branch.
func Test_branchTrueByteCode_StringFalse_NoBranch(t *testing.T) {
	tc := newTestContext(t).
		withBytecodeSize(10).
		withTypeStrictness(defs.NoTypeEnforcement).
		withStack("false")

	err := branchTrueByteCode(tc.ctx, 2)

	tc.assertNoError(err)
	tc.assertProgramCounter(0)
}

// Test_branchTrueByteCode_StackUnderflow verifies ErrStackUnderflow on an
// empty stack.
func Test_branchTrueByteCode_StackUnderflow(t *testing.T) {
	tc := newTestContext(t).withBytecodeSize(10)

	err := branchTrueByteCode(tc.ctx, 5)

	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_branchTrueByteCode_StrictMode_NonBool_Rejected verifies that a
// non-boolean TOS is rejected in strict mode.
func Test_branchTrueByteCode_StrictMode_NonBool_Rejected(t *testing.T) {
	tc := newTestContext(t).
		withBytecodeSize(10).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withStack(1)

	err := branchTrueByteCode(tc.ctx, 5)

	tc.assertError(err, errors.ErrConditionalBool)
}

// Test_branchTrueByteCode_StrictMode_Bool_Accepted verifies that strict mode
// accepts a genuine boolean TOS.
func Test_branchTrueByteCode_StrictMode_Bool_Accepted(t *testing.T) {
	tc := newTestContext(t).
		withBytecodeSize(10).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withStack(true)

	err := branchTrueByteCode(tc.ctx, 5)

	tc.assertNoError(err)
	tc.assertProgramCounter(5) // true → branch taken
}

// Test_branchTrueByteCode_InvalidAddress_Negative verifies that a negative
// address is rejected.
func Test_branchTrueByteCode_InvalidAddress_Negative(t *testing.T) {
	tc := newTestContext(t).
		withBytecodeSize(10).
		withStack(true)

	err := branchTrueByteCode(tc.ctx, -1)

	tc.assertError(err, errors.ErrInvalidBytecodeAddress)
}

// Test_branchTrueByteCode_InvalidAddress_TooLarge verifies that an address
// beyond nextAddress is rejected.
func Test_branchTrueByteCode_InvalidAddress_TooLarge(t *testing.T) {
	tc := newTestContext(t).
		withBytecodeSize(5).
		withStack(true)

	err := branchTrueByteCode(tc.ctx, 99)

	tc.assertError(err, errors.ErrInvalidBytecodeAddress)
}

// Test_branchTrueByteCode_StackPreservedOnAddressError mirrors the branchFalse
// equivalent: after the BRANCH-1 fix the condition value is preserved on the
// stack when the address is invalid.
func Test_branchTrueByteCode_StackPreservedOnAddressError(t *testing.T) {
	tc := newTestContext(t).
		withBytecodeSize(5).
		withStack(true)

	// Invalid address — the instruction errors before it ever pops TOS.
	_ = branchTrueByteCode(tc.ctx, 99)

	// The condition value must still be on the stack.
	tc.assertTopStack(true)
}

// ─── Section 4: Shared address-validation edge cases ─────────────────────────

// Test_branch_AddressBoundary_AllThree verifies that all three branch
// instructions accept an address equal to nextAddress (the shared upper bound).
func Test_branch_AddressBoundary_AllThree(t *testing.T) {
	for _, fn := range []struct {
		name string
		call func(*Context, any) error
	}{
		{"branchByteCode", branchByteCode},
		{"branchFalseByteCode", branchFalseByteCode},
		{"branchTrueByteCode", branchTrueByteCode},
	} {
		t.Run(fn.name, func(t *testing.T) {
			tc := newTestContext(t).withBytecodeSize(4)
			if fn.name != "branchByteCode" {
				tc.withStack(false) // provide a TOS value for the conditional branches
			}

			err := fn.call(tc.ctx, 4) // 4 == nextAddress → valid

			tc.assertNoError(err)
		})
	}
}

// Test_branch_NonIntOperand_AllThree verifies that all three branch
// instructions reject a non-convertible operand with ErrInvalidBytecodeAddress.
func Test_branch_NonIntOperand_AllThree(t *testing.T) {
	for _, fn := range []struct {
		name string
		call func(*Context, any) error
	}{
		{"branchByteCode", branchByteCode},
		{"branchFalseByteCode", branchFalseByteCode},
		{"branchTrueByteCode", branchTrueByteCode},
	} {
		t.Run(fn.name, func(t *testing.T) {
			tc := newTestContext(t).withBytecodeSize(10)
			if fn.name != "branchByteCode" {
				tc.withStack(false)
			}

			err := fn.call(tc.ctx, "not-a-number")

			tc.assertError(err, errors.ErrInvalidBytecodeAddress)
		})
	}
}
