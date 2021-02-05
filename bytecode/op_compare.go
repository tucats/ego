package bytecode

import (
	"reflect"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/util"
)

// EqualImpl instruction processor.
func EqualImpl(c *Context, i interface{}) error {
	// Terms pushed in reverse order
	v2, err := c.Pop()
	if err != nil {
		return err
	}

	v1, err := c.Pop()
	if err != nil {
		return err
	}

	var r bool

	switch a := v1.(type) {
	case nil:
		r = (v2 == nil)

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

// NotEqualImpl instruction processor.
func NotEqualImpl(c *Context, i interface{}) error {
	// Terms pushed in reverse order
	v2, err := c.Pop()
	if err != nil {
		return err
	}

	v1, err := c.Pop()
	if err != nil {
		return err
	}

	var r bool
	switch v1.(type) {
	case nil:
		r = (v2 != nil)

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

// GreaterThanImpl instruction processor.
func GreaterThanImpl(c *Context, i interface{}) error {
	// Terms pushed in reverse order
	v2, err := c.Pop()
	if err != nil {
		return err
	}

	v1, err := c.Pop()
	if err != nil {
		return err
	}

	if v1 == nil || v2 == nil {
		_ = c.Push(false)

		return nil
	}

	var r bool

	switch v1.(type) {
	case []interface{}, *datatypes.EgoMap, *datatypes.EgoArray:
		return c.NewError(InvalidTypeError)

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
			return c.NewError(InvalidTypeError)
		}
	}

	_ = c.Push(r)

	return nil
}

// GreaterThanOrEqualImpl instruction processor.
func GreaterThanOrEqualImpl(c *Context, i interface{}) error {
	// Terms pushed in reverse order
	v2, err := c.Pop()
	if err != nil {
		return err
	}

	v1, err := c.Pop()
	if err != nil {
		return err
	}

	if v1 == nil || v2 == nil {
		_ = c.Push(false)

		return nil
	}

	var r bool

	switch v1.(type) {
	case []interface{}, *datatypes.EgoMap, *datatypes.EgoArray:
		return c.NewError(InvalidTypeError)

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
			return c.NewError(InvalidTypeError)
		}
	}

	_ = c.Push(r)

	return nil
}

// LessThanImpl instruction processor.
func LessThanImpl(c *Context, i interface{}) error {
	// Terms pushed in reverse order
	v2, err := c.Pop()
	if err != nil {
		return err
	}

	v1, err := c.Pop()
	if err != nil {
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
		return c.NewError(InvalidTypeError)

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
			return c.NewError(InvalidTypeError)
		}
	}

	_ = c.Push(r)

	return nil
}

// LessThanOrEqualImpl instruction processor.
func LessThanOrEqualImpl(c *Context, i interface{}) error {
	// Terms pushed in reverse order.
	v2, err := c.Pop()
	if err != nil {
		return err
	}

	v1, err := c.Pop()
	if err != nil {
		return err
	}

	if v1 == nil || v2 == nil {
		_ = c.Push(false)

		return nil
	}

	var r bool

	switch v1.(type) {
	case []interface{}, *datatypes.EgoMap, *datatypes.EgoArray:
		return c.NewError(InvalidTypeError)

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
			return c.NewError(InvalidTypeError)
		}
	}

	_ = c.Push(r)

	return nil
}
