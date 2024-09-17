package bytecode

import (
	"reflect"

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
	case *data.Type:
		if v, ok := v2.(string); ok {
			result = (actual.String() == v)
		} else if v, ok := v2.(*data.Type); ok {
			// Deep equal gets goobered up with types that have
			// pointers, so let's conver to string values and compare
			// the strings.
			t1 := actual.String()
			t2 := v.String()
			result = (t1 == t2)
		} else {
			return errors.ErrNotAType.Context(v2)
		}

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
		if v1 == nil && v2 == nil {
			result = true
		} else {
			switch v1.(type) {
			case nil:
				result = false

			case byte, int32, int, int64:
				result = data.Int64(v1) == data.Int64(v2)

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
	}

	_ = c.push(result)

	return nil
}

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

	// IF only one side is nil, they are not equal by definition.
	if !data.IsNil(v1) && data.IsNil(v2) ||
		data.IsNil(v1) && !data.IsNil(v2) {
		return c.push(true)
	}

	var result bool

	switch actual := v1.(type) {
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
			v1, v2 = data.Normalize(v1, v2)
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

// lessThanByteCode implements the LessThan opcode
//
// Inputs:
//
//	stack+0    - The item to be compared
//	stack+1    - The item to compare to
//
// The top two values are popped from the stack,
// and a type-specific test for equality is done.
// If the top value is less than the second
// value, then true is pushed back on the stack,
// else false.
func lessThanByteCode(c *Context, i interface{}) error {
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

	// Handle nil cases
	if v1 == nil || v2 == nil {
		_ = c.push(false)

		return nil
	}

	// Nope, going to have to do type-sensitive compares.
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
			result = data.Int64(v1) < data.Int64(v2)

		case float32:
			result = v1.(float32) < v2.(float32)

		case float64:
			result = v1.(float64) < v2.(float64)

		case string:
			result = v1.(string) < v2.(string)

		default:
			return c.error(errors.ErrInvalidType).Context(data.TypeOf(v1).String())
		}
	}

	_ = c.push(result)

	return nil
}

// lessThanOrEqualByteCode implements the LessThanOrEqual
// opcode
//
// Inputs:
//
//	stack+0    - The item to be compared
//	stack+1    - The item to compare to
//
// The top two values are popped from the stack,
// and a type-specific test for equality is done.
// If the top value is less than or equal to the
// second value, then true is pushed back on the
// stack, else false.
func lessThanOrEqualByteCode(c *Context, i interface{}) error {
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

	// Unwrap the type using a switch to handle the cases that are invalid
	// for this kind of comparison. The default case is where the real work
	// is done.
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
			result = data.Int64(v1) <= data.Int64(v2)

		case float32:
			result = v1.(float32) <= v2.(float32)

		case float64:
			result = v1.(float64) <= v2.(float64)

		case string:
			result = v1.(string) <= v2.(string)

		default:
			return c.error(errors.ErrInvalidType).Context(data.TypeOf(v1).String())
		}
	}

	_ = c.push(result)

	return nil
}
