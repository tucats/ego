package bytecode

import (
	"reflect"
	"time"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

// equalByteCode implements the Equal opcode
//
// Inputs:
//
//	stack+0    - The item to be compared
//	stack+1    - The item to compare to
//
// The top two values are popped from the stack,
// and a type-specific test for equality is done.
// If the values are equal, then true is pushed
// back on the stack, else false.
func equalByteCode(c *Context, i interface{}) error {
	var err error

	// Get the two terms to compare. These are found either in the operand as an
	// array of values or on the stack.
	v1, v2, err := getComparisonTerms(c, i)
	if err != nil {
		return err
	}

	// If both are nil, then they match.
	if data.IsNil(v1) && data.IsNil(v2) {
		return c.push(true)
	}

	// Otherwise, if either one is nil, there is no match
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.push(false)
	}

	var result bool

	switch actual := v1.(type) {
	case time.Duration:
		if d, ok := v2.(time.Duration); ok {
			result = (actual == d)
		} else {
			return c.runtimeError(errors.ErrInvalidTypeForOperation)
		}

	case time.Time:
		if d, ok := v2.(time.Time); ok {
			result = (actual.Equal(d))
		} else {
			return c.runtimeError(errors.ErrInvalidTypeForOperation)
		}

	case *data.Type:
		return equalTypes(v2, c, actual)

	case nil:
		if err, ok := v2.(error); ok {
			result = errors.Nil(err)
		} else {
			result = (v2 == nil)
		}

	case *errors.Error:
		result = actual.Equal(v2)

	case *data.Struct:
		str, ok := v2.(*data.Struct)
		if ok {
			result = reflect.DeepEqual(actual, str)
		} else {
			result = false
		}

	case *data.Map:
		result = reflect.DeepEqual(v1, v2)

	case *data.Array:
		if array, ok := v2.(*data.Array); ok {
			result = actual.DeepEqual(array)
		} else {
			result = false
		}

	default:
		return genericEqualCompare(c, v1, v2)
	}

	return c.push(result)
}

func genericEqualCompare(c *Context, v1 interface{}, v2 interface{}) error {
	var (
		err    error
		result bool
	)

	// If type checking is set to strict, the types must match exactly.
	if c.typeStrictness == defs.StrictTypeEnforcement {
		if !data.TypeOf(v1).IsType(data.TypeOf(v2)) {
			return c.runtimeError(errors.ErrTypeMismatch).
				Context(data.TypeOf(v2).String() + ", " + data.TypeOf(v1).String())
		}
	} else {
		// Otherwise, normalize the types to the same type.
		v1, v2, err = data.Normalize(v1, v2)
		if err != nil {
			return err
		}
	}

	if v1 == nil && v2 == nil {
		result = true
	} else {
		// Based on the now-normalized types, do the comparison.
		switch v1.(type) {
		case nil:
			result = false

		case byte, int32, int, int64:
			x1, err := data.Int64(v1)
			if err != nil {
				return err
			}

			x2, err := data.Int64(v2)
			if err != nil {
				return err
			}

			result = x1 == x2

		case float64:
			result = v1.(float64) == v2.(float64)

		case float32:
			result = v1.(float32) == v2.(float32)

		case string:
			result = v1.(string) == v2.(string)

		case bool:
			result = v1.(bool) == v2.(bool)
		}
	}

	return c.push(result)
}

// Compare the v2 value with the actual type. Because deep equal testing cannot
// be used, we attempt to format the types as strings and compare the strings.
func equalTypes(v2 interface{}, c *Context, actual *data.Type) error {
	if v, ok := v2.(string); ok {
		return c.push(actual.String() == v)
	} else if v, ok := v2.(*data.Type); ok {
		t1 := actual.String()
		t2 := v.String()

		return c.push(t1 == t2)
	}

	return errors.ErrNotAType.Context(v2)
}

func getComparisonTerms(c *Context, i interface{}) (interface{}, interface{}, error) {
	var (
		err error
		v1  interface{}
		v2  interface{}
	)

	if array, ok := i.([]interface{}); ok && len(array) == 1 {
		v2 = array[0]
		if constant, ok := v2.(data.Immutable); ok {
			v2 = constant.Value
		}
	} else {
		v2, err = c.Pop()
		if err != nil {
			return nil, nil, err
		}
	}

	v1, err = c.Pop()
	if err != nil {
		return nil, nil, err
	}

	// If either value is a stack marker, then this is an error, typically
	// because a function returned a void value and didn't leave anything on
	// the stack.
	if isStackMarker(v1) || isStackMarker(v2) {
		return nil, nil, c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	return v1, v2, nil
}
