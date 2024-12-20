package bytecode

import (
	"reflect"
	"time"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

// notEqualByteCode implements the NotEqual opcode
//
// Inputs:
//
//	stack+0    - The item to be compared
//	stack+1    - The item to compare to
//
// The top two values are popped from the stack,
// and a type-specific test for equality is done.
// If the values are not equal, then true is pushed
// back on the stack, else false.
func notEqualByteCode(c *Context, i interface{}) error {
	var err error
	// Get the two terms to compare. These are found either in the operand as an
	// array of values or on the stack.
	v1, v2, err := getComparisonTerms(c, i)
	if err != nil {
		return err
	}

	// IF only one side is nil, they are not equal by definition.
	if !data.IsNil(v1) && data.IsNil(v2) ||
		data.IsNil(v1) && !data.IsNil(v2) {
		return c.push(true)
	}

	var result bool

	switch actual := v1.(type) {
	case time.Duration:
		if d, ok := v2.(time.Duration); ok {
			result = (actual != d)
		} else {
			return c.error(errors.ErrInvalidTypeForOperation)
		}

	case time.Time:
		if d, ok := v2.(time.Time); ok {
			result = !(actual.Equal(d))
		} else {
			return c.error(errors.ErrInvalidTypeForOperation)
		}

	case nil:
		result = (v2 != nil)

	case *errors.Error:
		result = !actual.Equal(v2)

	case error:
		result = !reflect.DeepEqual(v1, v2)

	case data.Map:
		result = !reflect.DeepEqual(v1, v2)

	case data.Array:
		result = !reflect.DeepEqual(v1, v2)

	case data.Struct:
		result = !reflect.DeepEqual(v1, v2)

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
		case nil:
			result = false

		case byte, int32, int, int64:
			result = data.Int64(v1) != data.Int64(v2)

		case float32:
			result = v1.(float32) != v2.(float32)

		case float64:
			result = v1.(float64) != v2.(float64)

		case string:
			result = v1.(string) != v2.(string)

		case bool:
			result = v1.(bool) != v2.(bool)
		}
	}

	_ = c.push(result)

	return nil
}
