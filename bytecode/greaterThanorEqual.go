package bytecode

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

// greaterThanOrEqualByteCode implements the GreaterThanOrEqual
//
//	opcode
//
// Inputs:
//
//	stack+0    - The item to be compared
//	stack+1    - The item to compare to
//
// The top two values are popped from the stack,
// and a type-specific test for equality is done.
// If the top value is greater than or equal to the
// second value, then true is pushed back on the stack,
// else false.
func greaterThanOrEqualByteCode(c *Context, i interface{}) error {
	var err error

	// Get the two terms to compare. These are found either in the operand as an
	// array of values or on the stack.
	v1, v2, err := getComparisonTerms(c, i)
	if err != nil {
		return err
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
			v1, v2, err = data.Normalize(v1, v2)
			if err != nil {
				return err
			}
		}

		// Based on the now-normalized types, do the comparison.
		switch v1.(type) {
		case byte, int32, int, int64:
			result = data.Int64(v1) >= data.Int64(v2)

		case float32:
			result = v1.(float32) >= v2.(float32)

		case float64:
			result = v1.(float64) >= v2.(float64)

		case string:
			result = v1.(string) >= v2.(string)

		default:
			return c.error(errors.ErrInvalidType).Context(data.TypeOf(v1).String())
		}
	}

	_ = c.push(result)

	return nil
}
