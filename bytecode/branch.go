package bytecode

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

// branchFalseByteCode instruction processor branches to the instruction named in
// the operand if the top-of-stack item is a boolean FALSE value. Otherwise,
// execution continues with the next instruction.
//
// Parameters:
//
//		c	execution context
//		i	instruction operand is integer destiation bytecode address
//	[tos]	value to test to determine if branch is taken.
//
// Returns:
//
//	error	if any error occurs during execution, else nil
func branchFalseByteCode(c *Context, i interface{}) error {
	// Get test value
	v, err := c.Pop()
	if err != nil {
		return err
	}

	if c.typeStrictness == defs.StrictTypeEnforcement {
		if _, ok := v.(bool); !ok {
			return c.error(errors.ErrConditionalBool).Context(data.TypeOf(v).String())
		}
	}

	// Get destination. If it is out of range, that's an error.
	if address, err := data.Int(i); err != nil || address < 0 || address > c.bc.nextAddress {
		return c.error(errors.ErrInvalidBytecodeAddress).Context(address)
	} else {
		if b, err := data.Bool(v); err != nil {
			return c.error(err)
		} else if !b {
			c.programCounter = address
		}
	}

	return nil
}

// branchByteCode instruction processor branches to the instruction named in
// the operand.
//
// Parameters:
//
//		c	execution context
//		i	instruction operand is integer destiation bytecode address
//	[tos]	value to test to determine if branch is taken.
//
// Returns:
//
//	error	if any error occurs during execution, else nil
func branchByteCode(c *Context, i interface{}) error {
	// Get destination
	if address, err := data.Int(i); err != nil || address < 0 || address > c.bc.nextAddress {
		return c.error(errors.ErrInvalidBytecodeAddress).Context(address)
	} else {
		c.programCounter = address
	}

	return nil
}

// branchTrueByteCode instruction processor branches to the instruction named in
// the operand if the top-of-stack item is a boolean TRUE value. Otherwise,
// execution continues with the next instruction.
//
// Parameters:
//
//		c	execution context
//		i	instruction operand is integer destiation bytecode address
//	[tos]	value to test to determine if branch is taken.
//
// Returns:
//
//	error	if any error occurs during execution, else nil
func branchTrueByteCode(c *Context, i interface{}) error {
	// Get test value
	v, err := c.Pop()
	if err != nil {
		return err
	}

	// If we are doing strict type checking, then the value must be a boolean.
	if c.typeStrictness == defs.StrictTypeEnforcement {
		if _, ok := v.(bool); !ok {
			return c.error(errors.ErrConditionalBool).Context(data.TypeOf(v).String())
		}
	}

	// Get destination
	if address, err := data.Int(i); err != nil || address < 0 || address > c.bc.nextAddress {
		return c.error(errors.ErrInvalidBytecodeAddress).Context(address)
	} else {
		if b, err := data.Bool(v); err != nil {
			return c.error(err)
		} else if b {
			c.programCounter = address
		}
	}

	return nil
}
