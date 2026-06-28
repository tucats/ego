package bytecode

import (
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// validateBranchAddress converts and validates the branch-target operand i
// that is shared by every branch instruction.  It is extracted into its own
// method so that the three fixes below live in one place rather than being
// duplicated across branchByteCode, branchFalseByteCode, and branchTrueByteCode.
//
// The three problems it fixes (see docs/BYTECODE_ISSUES.md):
//
//   - BRANCH-2: data.Int(nil) returns (0, nil), so a nil operand was silently
//     accepted as "branch to address 0".  This masked compiler bugs where a
//     branch operand was accidentally left nil.  The fix is an explicit nil
//     check that returns ErrInvalidBytecodeAddress before calling data.Int.
//
//   - BRANCH-3: when data.Int(i) failed because i was not convertible to an
//     integer, the error context showed 0 (the zero-value that data.Int returns
//     on failure) rather than the actual bad operand.  The fix separates the
//     conversion failure from the bounds check, and uses the original operand i
//     as the context value when conversion fails.
//
// Valid addresses are in the range [0, bc.nextAddress].  The upper bound is
// inclusive so that a branch to the instruction just past the last opcode (the
// implicit function-exit point) is legal.
func (c *Context) validateBranchAddress(i any) (int, error) {
	// Reject nil explicitly before calling data.Int.  data.Int(nil) returns
	// (0, nil), so without this guard a nil operand would silently branch to
	// address 0 instead of signalling a programming error.
	if i == nil {
		return 0, c.runtimeError(errors.ErrInvalidBytecodeAddress).Context("nil")
	}

	// Convert the operand to an integer address.  Keep a reference to the
	// original operand i so that if conversion fails the error message names
	// the actual bad value (e.g. "not-a-number") rather than the zero value
	// that data.Int returns on failure.
	address, err := data.Int(i)
	if err != nil {
		return 0, c.runtimeError(errors.ErrInvalidBytecodeAddress).Context(i)
	}

	// Range check: the address must be within [0, nextAddress].
	if address < 0 || address > c.bc.nextAddress {
		return 0, c.runtimeError(errors.ErrInvalidBytecodeAddress).Context(address)
	}

	return address, nil
}

// branchByteCode is the unconditional branch instruction.  It sets the program
// counter to the address supplied as the operand, causing the next instruction
// to be fetched from that location.
//
// The stack is not modified: no items are consumed or produced.
//
// Parameters:
//
//	c  execution context
//	i  operand: integer branch-target address
//
// Returns an error if the address is invalid (nil, non-integer, or out of range).
func branchByteCode(c *Context, i any) error {
	address, err := c.validateBranchAddress(i)
	if err != nil {
		return err
	}

	c.programCounter = address

	return nil
}

// branchFalseByteCode is the conditional branch instruction that branches when
// the top-of-stack value is falsy.  It pops one item from the stack and
// evaluates it as a boolean.  If the boolean is false, the program counter is
// set to the target address; otherwise execution continues at the next
// instruction.
//
// Strict-mode note: when type strictness is StrictTypeEnforcement the TOS
// value must be a native Go bool.  Any other type returns ErrConditionalBool.
// In relaxed and dynamic modes any value that data.Bool can convert is accepted.
//
// Order of operations (important for stack safety — see docs/BYTECODE_ISSUES.md BRANCH-1):
//
//  1. Validate the branch address (no stack side effects).
//  2. Pop TOS — only reached when the address is known valid.
//  3. Enforce strict-mode type constraint if active.
//  4. Coerce to bool and conditionally set the program counter.
//
// Parameters:
//
//	c     execution context
//	i     operand: integer branch-target address
//
// [tos]  the condition value; consumed by this instruction.
//
// Returns an error if the address is invalid, the stack is empty, or in strict
// mode when TOS is not a boolean.
func branchFalseByteCode(c *Context, i any) error {
	// Validate the address first.  In the original code the Pop happened
	// before this check, which meant an invalid address would consume TOS
	// and leave the stack in an unexpected state if the error was caught by
	// an Ego try/catch block.  Validating first ensures the stack is never
	// modified unless the instruction is going to succeed or fail on the
	// condition itself.
	address, err := c.validateBranchAddress(i)
	if err != nil {
		return err
	}

	// Pop the condition value.  The address is valid, so it is now safe to
	// consume the top-of-stack.
	v, err := c.Pop()
	if err != nil {
		return err
	}

	// In strict mode the condition must be a native bool, not an integer or
	// other type that merely coerces to bool.  This catches type mismatches
	// early with a clear error rather than silently accepting 0 as false.
	if c.typeStrictness == defs.StrictTypeEnforcement {
		if _, ok := v.(bool); !ok {
			return c.runtimeError(errors.ErrConditionalBool).Context(data.TypeOf(v).String())
		}
	}

	// Coerce to bool.  In non-strict mode this handles integers, strings, and
	// other types that have a natural boolean interpretation.
	b, err := data.Bool(v)
	if err != nil {
		return c.runtimeError(err)
	}

	// Take the branch only when the condition is false.
	if !b {
		c.programCounter = address
	}

	return nil
}

// branchTrueByteCode is the conditional branch instruction that branches when
// the top-of-stack value is truthy.  It is the logical complement of
// branchFalseByteCode: all semantics are identical except the branch fires
// when the condition evaluates to true rather than false.
//
// Order of operations (see docs/BYTECODE_ISSUES.md BRANCH-1):
//
//  1. Validate the branch address (no stack side effects).
//  2. Pop TOS — only reached when the address is known valid.
//  3. Enforce strict-mode type constraint if active.
//  4. Coerce to bool and conditionally set the program counter.
//
// Parameters:
//
//	c     execution context
//	i     operand: integer branch-target address
//
// [tos]  the condition value; consumed by this instruction.
//
// Returns an error if the address is invalid, the stack is empty, or in strict
// mode when TOS is not a boolean.
func branchTrueByteCode(c *Context, i any) error {
	// Validate the address before touching the stack (BRANCH-1 fix).
	address, err := c.validateBranchAddress(i)
	if err != nil {
		return err
	}

	// Pop the condition value only once the address is confirmed valid.
	v, err := c.Pop()
	if err != nil {
		return err
	}

	// Strict-mode type check: the condition must be a native bool.
	if c.typeStrictness == defs.StrictTypeEnforcement {
		if _, ok := v.(bool); !ok {
			return c.runtimeError(errors.ErrConditionalBool).Context(data.TypeOf(v).String())
		}
	}

	// Coerce to bool and branch when true.
	b, err := data.Bool(v)
	if err != nil {
		return c.runtimeError(err)
	}

	if b {
		c.programCounter = address
	}

	return nil
}
