package bytecode

import (
	"reflect"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// EqualImpl implements the Equal opcode
//
// Inputs:
//    stack+0    - The item to be compared
//    stack+1    - The item to compare to
//
// The top two values are popped from the stack,
// and a type-specific test for equality is done.
// If the values are equal, then true is pushed
// back on the stack, else false.
func EqualImpl(c *Context, i interface{}) *errors.EgoError {
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
		return c.Push(true)
	}

	// Otherwise, if either one is nil, there is no match
	if datatypes.IsNil(v1) || datatypes.IsNil(v2) {
		return c.Push(false)
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

	case map[string]interface{}:
		r = reflect.DeepEqual(v1, v2)

	case *datatypes.EgoMap:
		r = reflect.DeepEqual(v1, v2)

	case *datatypes.EgoArray:
		switch b := v2.(type) {
		case *datatypes.EgoArray:
			r = a.DeepEqual(b)

		case []interface{}:
			r = reflect.DeepEqual(a.BaseArray(), b)

		default:
			r = false
		}

	case []interface{}:
		r = reflect.DeepEqual(v1, v2)

	default:
		v1, v2 = util.Normalize(v1, v2)
		if v1 == nil && v2 == nil {
			r = true
		} else {
			switch v1.(type) {
			case nil:
				r = false

			case int:
				r = v1.(int) == v2.(int)

			case float64:
				r = v1.(float64) == v2.(float64)

			case string:
				r = v1.(string) == v2.(string)

			case bool:
				r = v1.(bool) == v2.(bool)
			}
		}
	}

	_ = c.Push(r)

	return nil
}

// NotEqualImpl implements the NotEqual opcode
//
// Inputs:
//    stack+0    - The item to be compared
//    stack+1    - The item to compare to
//
// The top two values are popped from the stack,
// and a type-specific test for equality is done.
// If the values are not equal, then true is pushed
// back on the stack, else false.
func NotEqualImpl(c *Context, i interface{}) *errors.EgoError {
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
		return c.Push(true)
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

	case map[string]interface{}:
		r = !reflect.DeepEqual(v1, v2)

	case []interface{}:
		r = !reflect.DeepEqual(v1, v2)

	default:
		v1, v2 = util.Normalize(v1, v2)

		switch v1.(type) {
		case nil:
			r = false

		case int:
			r = v1.(int) != v2.(int)

		case float64:
			r = v1.(float64) != v2.(float64)

		case string:
			r = v1.(string) != v2.(string)

		case bool:
			r = v1.(bool) != v2.(bool)
		}
	}

	_ = c.Push(r)

	return nil
}

// GreaterThanImpl implements the GreaterThan opcode
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
func GreaterThanImpl(c *Context, i interface{}) *errors.EgoError {
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
		_ = c.Push(false)

		return nil
	}

	var r bool

	switch v1.(type) {
	case []interface{}, *datatypes.EgoMap, *datatypes.EgoArray:
		return c.NewError(errors.InvalidTypeError)

	default:
		v1, v2 = util.Normalize(v1, v2)

		switch v1.(type) {
		case int:
			r = v1.(int) > v2.(int)

		case float64:
			r = v1.(float64) > v2.(float64)

		case string:
			r = v1.(string) > v2.(string)

		default:
			return c.NewError(errors.InvalidTypeError)
		}
	}

	_ = c.Push(r)

	return nil
}

// GreaterThanOrEqualImpl implements the GreaterThanOrEqual
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
func GreaterThanOrEqualImpl(c *Context, i interface{}) *errors.EgoError {
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
		_ = c.Push(false)

		return nil
	}

	var r bool

	switch v1.(type) {
	case []interface{}, *datatypes.EgoMap, *datatypes.EgoArray:
		return c.NewError(errors.InvalidTypeError)

	default:
		v1, v2 = util.Normalize(v1, v2)

		switch v1.(type) {
		case int:
			r = v1.(int) >= v2.(int)

		case float64:
			r = v1.(float64) >= v2.(float64)

		case string:
			r = v1.(string) >= v2.(string)

		default:
			return c.NewError(errors.InvalidTypeError)
		}
	}

	_ = c.Push(r)

	return nil
}

// LessThanImpl implements the LessThan opcode
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
func LessThanImpl(c *Context, i interface{}) *errors.EgoError {
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
		_ = c.Push(false)

		return nil
	}

	// Nope, going to have to do type-sensitive compares.
	var r bool

	switch v1.(type) {
	case []interface{}, *datatypes.EgoMap, *datatypes.EgoArray:
		return c.NewError(errors.InvalidTypeError)

	default:
		v1, v2 = util.Normalize(v1, v2)

		switch v1.(type) {
		case int:
			r = v1.(int) < v2.(int)

		case float64:
			r = v1.(float64) < v2.(float64)

		case string:
			r = v1.(string) < v2.(string)

		default:
			return c.NewError(errors.InvalidTypeError)
		}
	}

	_ = c.Push(r)

	return nil
}

// LessThanOrEqualImpl implements the LessThanOrEqual
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
func LessThanOrEqualImpl(c *Context, i interface{}) *errors.EgoError {
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
		_ = c.Push(false)

		return nil
	}

	var r bool

	switch v1.(type) {
	case []interface{}, *datatypes.EgoMap, *datatypes.EgoArray:
		return c.NewError(errors.InvalidTypeError)

	default:
		v1, v2 = util.Normalize(v1, v2)
		switch v1.(type) {
		case int:
			r = v1.(int) <= v2.(int)

		case float64:
			r = v1.(float64) <= v2.(float64)

		case string:
			r = v1.(string) <= v2.(string)

		default:
			return c.NewError(errors.InvalidTypeError)
		}
	}

	_ = c.Push(r)

	return nil
}
