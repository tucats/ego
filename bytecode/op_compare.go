package bytecode

import (
	"reflect"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// equalByteCode implements the Equal opcode
//
// Inputs:
//    stack+0    - The item to be compared
//    stack+1    - The item to compare to
//
// The top two values are popped from the stack,
// and a type-specific test for equality is done.
// If the values are equal, then true is pushed
// back on the stack, else false.
func equalByteCode(c *Context, i interface{}) *errors.EgoError {
	// Terms pushed in reverse order
	v2, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	v1, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	// If both are nil, then they match.
	if datatypes.IsNil(v1) && datatypes.IsNil(v2) {
		return c.stackPush(true)
	}

	// Otherwise, if either one is nil, there is no match
	if datatypes.IsNil(v1) || datatypes.IsNil(v2) {
		return c.stackPush(false)
	}

	var r bool

	switch a := v1.(type) {
	case nil:
		if e2, ok := v2.(*errors.EgoError); ok {
			r = errors.Nil(e2)
		} else {
			r = (v2 == nil)
		}

	case *errors.EgoError:
		r = a.Equal(v2)

	case *datatypes.EgoStruct:
		a2, ok := v2.(*datatypes.EgoStruct)
		if ok {
			r = reflect.DeepEqual(a, a2)
		} else {
			r = false
		}

	case *datatypes.EgoMap:
		r = reflect.DeepEqual(v1, v2)

	case *datatypes.EgoArray:
		switch b := v2.(type) {
		case *datatypes.EgoArray:
			r = a.DeepEqual(b)

		default:
			r = false
		}

	default:
		v1, v2 = util.Normalize(v1, v2)
		if v1 == nil && v2 == nil {
			r = true
		} else {
			switch v1.(type) {
			case nil:
				r = false

			case byte, int32, int, int64:
				r = util.GetInt(v1) == util.GetInt(v2)

			case float64:
				r = v1.(float64) == v2.(float64)

			case float32:
				r = v1.(float32) == v2.(float32)

			case string:
				r = v1.(string) == v2.(string)

			case bool:
				r = v1.(bool) == v2.(bool)
			}
		}
	}

	_ = c.stackPush(r)

	return nil
}

// notEqualByteCode implements the NotEqual opcode
//
// Inputs:
//    stack+0    - The item to be compared
//    stack+1    - The item to compare to
//
// The top two values are popped from the stack,
// and a type-specific test for equality is done.
// If the values are not equal, then true is pushed
// back on the stack, else false.
func notEqualByteCode(c *Context, i interface{}) *errors.EgoError {
	// Terms pushed in reverse order
	v2, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	v1, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	// IF only one side is nil, they are not equal by definition.
	if !datatypes.IsNil(v1) && datatypes.IsNil(v2) ||
		datatypes.IsNil(v1) && !datatypes.IsNil(v2) {
		return c.stackPush(true)
	}

	var r bool

	switch a := v1.(type) {
	case nil:
		if e2, ok := v2.(*errors.EgoError); ok {
			r = !errors.Nil(e2)
		} else {
			r = (v2 != nil)
		}

	case *errors.EgoError:
		r = !a.Equal(v2)

	case error:
		r = !reflect.DeepEqual(v1, v2)

	case *datatypes.EgoMap:
		r = !reflect.DeepEqual(v1, v2)

	case *datatypes.EgoArray:
		r = !reflect.DeepEqual(v1, v2)

	case *datatypes.EgoStruct:
		r = !reflect.DeepEqual(v1, v2)

	default:
		v1, v2 = util.Normalize(v1, v2)

		switch v1.(type) {
		case nil:
			r = false

		case byte, int32, int, int64:
			r = util.GetInt(v1) != util.GetInt(v2)

		case float32:
			r = v1.(float32) != v2.(float32)

		case float64:
			r = v1.(float64) != v2.(float64)

		case string:
			r = v1.(string) != v2.(string)

		case bool:
			r = v1.(bool) != v2.(bool)
		}
	}

	_ = c.stackPush(r)

	return nil
}

// greaterThanByteCode implements the GreaterThan opcode
//
// Inputs:
//    stack+0    - The item to be compared
//    stack+1    - The item to compare to
//
// The top two values are popped from the stack,
// and a type-specific test for equality is done.
// If the top value is greater than the second
// value, then true is pushed back on the stack,
// else false.
func greaterThanByteCode(c *Context, i interface{}) *errors.EgoError {
	// Terms pushed in reverse order
	v2, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	v1, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	if v1 == nil || v2 == nil {
		_ = c.stackPush(false)

		return nil
	}

	var r bool

	switch v1.(type) {
	case *datatypes.EgoMap, *datatypes.EgoStruct, datatypes.EgoPackage, *datatypes.EgoArray:
		return c.newError(errors.ErrInvalidType)

	default:
		v1, v2 = util.Normalize(v1, v2)

		switch v1.(type) {
		case byte, int32, int, int64:
			r = util.GetInt(v1) > util.GetInt(v2)

		case float32:
			r = v1.(float32) > v2.(float32)

		case float64:
			r = v1.(float64) > v2.(float64)

		case string:
			r = v1.(string) > v2.(string)

		default:
			return c.newError(errors.ErrInvalidType)
		}
	}

	_ = c.stackPush(r)

	return nil
}

// greaterThanOrEqualByteCode implements the GreaterThanOrEqual
//  opcode
//
// Inputs:
//    stack+0    - The item to be compared
//    stack+1    - The item to compare to
//
// The top two values are popped from the stack,
// and a type-specific test for equality is done.
// If the top value is greater than or equal to the
// second value, then true is pushed back on the stack,
// else false.
func greaterThanOrEqualByteCode(c *Context, i interface{}) *errors.EgoError {
	// Terms pushed in reverse order
	v2, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	v1, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	if v1 == nil || v2 == nil {
		_ = c.stackPush(false)

		return nil
	}

	var r bool

	switch v1.(type) {
	case *datatypes.EgoMap, *datatypes.EgoStruct, datatypes.EgoPackage, *datatypes.EgoArray:
		return c.newError(errors.ErrInvalidType)

	default:
		v1, v2 = util.Normalize(v1, v2)

		switch v1.(type) {
		case byte, int32, int, int64:
			r = util.GetInt(v1) >= util.GetInt(v2)

		case float32:
			r = v1.(float32) >= v2.(float32)

		case float64:
			r = v1.(float64) >= v2.(float64)

		case string:
			r = v1.(string) >= v2.(string)

		default:
			return c.newError(errors.ErrInvalidType)
		}
	}

	_ = c.stackPush(r)

	return nil
}

// lessThanByteCode implements the LessThan opcode
//
// Inputs:
//    stack+0    - The item to be compared
//    stack+1    - The item to compare to
//
// The top two values are popped from the stack,
// and a type-specific test for equality is done.
// If the top value is less than the second
// value, then true is pushed back on the stack,
// else false.
func lessThanByteCode(c *Context, i interface{}) *errors.EgoError {
	// Terms pushed in reverse order
	v2, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	v1, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	// Handle nil cases
	if v1 == nil || v2 == nil {
		_ = c.stackPush(false)

		return nil
	}

	// Nope, going to have to do type-sensitive compares.
	var r bool

	switch v1.(type) {
	case *datatypes.EgoMap, *datatypes.EgoStruct, datatypes.EgoPackage, *datatypes.EgoArray:
		return c.newError(errors.ErrInvalidType)

	default:
		v1, v2 = util.Normalize(v1, v2)

		switch v1.(type) {
		case byte, int32, int, int64:
			r = util.GetInt(v1) < util.GetInt(v2)

		case float32:
			r = v1.(float32) < v2.(float32)

		case float64:
			r = v1.(float64) < v2.(float64)

		case string:
			r = v1.(string) < v2.(string)

		default:
			return c.newError(errors.ErrInvalidType)
		}
	}

	_ = c.stackPush(r)

	return nil
}

// lessThanOrEqualByteCode implements the LessThanOrEqual
// opcode
//
// Inputs:
//    stack+0    - The item to be compared
//    stack+1    - The item to compare to
//
// The top two values are popped from the stack,
// and a type-specific test for equality is done.
// If the top value is less than or equal to the
// second value, then true is pushed back on the
// stack, else false.
func lessThanOrEqualByteCode(c *Context, i interface{}) *errors.EgoError {
	// Terms pushed in reverse order.
	v2, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	v1, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	if v1 == nil || v2 == nil {
		_ = c.stackPush(false)

		return nil
	}

	var r bool

	switch v1.(type) {
	case *datatypes.EgoMap, *datatypes.EgoStruct, datatypes.EgoPackage, *datatypes.EgoArray:
		return c.newError(errors.ErrInvalidType)

	default:
		v1, v2 = util.Normalize(v1, v2)
		switch v1.(type) {
		case byte, int32, int, int64:
			r = util.GetInt(v1) <= util.GetInt(v2)

		case float32:
			r = v1.(float32) <= v2.(float32)

		case float64:
			r = v1.(float64) <= v2.(float64)

		case string:
			r = v1.(string) <= v2.(string)

		default:
			return c.newError(errors.ErrInvalidType)
		}
	}

	_ = c.stackPush(r)

	return nil
}
