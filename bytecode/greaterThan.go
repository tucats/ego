package bytecode

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

// greaterThanByteCode implements the GreaterThan opcode
//
// Inputs:
//
//	stack+0    - The item to be compared
//	stack+1    - The item to compare to
//
// The top two values are popped from the stack,
// and a type-specific test for equality is done.
// If the top value is greater than the second
// value, then true is pushed back on the stack,
// else false.
func greaterThanByteCode(c *Context, i interface{}) error {
	var err error

	// Terms pushed in reverse order. If the operand contains an
	// interface array, we'll extract the item from it, else the
	// value is on the stack.
	var v2 interface{}

	if vv, ok := i.([]interface{}); ok && len(vv) == 1 {
		v2 = vv[0]
		if c, ok := v2.(data.Immutable); ok {
			v2 = c.Value
		}
	} else {
		v2, err = c.Pop()
		if err != nil {
			return err
		}
	}

	v1, err := c.Pop()
	if err != nil {
		return err
	}

	// If either value is a stack marker, then this is an error, typically
	// because a function returned a void value and didn't leave anything on
	// the stack.
	if isStackMarker(v1) || isStackMarker(v2) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	if v1 == nil || v2 == nil {
		_ = c.push(false)

		return nil
	}

	var result bool

	switch v1.(type) {
	case *data.Map, *data.Struct, *data.Package, *data.Array:
		return c.error(errors.ErrInvalidType).Context(data.TypeOf(v1).String())

	default:
		// If type checking is set to strict, the types must match exactly.
		if c.typeStrictness == defs.StrictTypeEnforcement {
			if !data.TypeOf(v1).IsType(data.TypeOf(v2)) {
				return c.error(errors.ErrTypeMismatch).
					Context(data.TypeOf(v2).String() + ", " + data.TypeOf(v1).String())
			}
		} else {
			// Otherwise, normalize the types to the same type.
			v1, v2 = data.Normalize(v1, v2)
		}

		// Based on the now-normalized types, do the comparison.
		switch v1.(type) {
		case byte, int32, int, int64:
			result = data.Int64(v1) > data.Int64(v2)

		case float32:
			result = v1.(float32) > v2.(float32)

		case float64:
			result = v1.(float64) > v2.(float64)

		case string:
			result = v1.(string) > v2.(string)

		default:
			return c.error(errors.ErrInvalidType).Context(data.TypeOf(v1).String())
		}
	}

	_ = c.push(result)

	return nil
}
