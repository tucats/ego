package bytecode

import (
	"reflect"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// StaticTypeOpcode implements the StaticType opcode, which
// sets the static typing flag for the current context.
func StaticTypingImpl(c *Context, i interface{}) *errors.EgoError {
	v, err := c.Pop()
	if errors.Nil(err) {
		c.Static = util.GetBool(v)
		err = c.symbols.SetAlways("__static_data_types", c.Static)
	}

	return err
}

func RequiredTypeImpl(c *Context, i interface{}) *errors.EgoError {
	v, err := c.Pop()
	if errors.Nil(err) {
		// If we're doing strict type checking...
		if c.Static {
			if t, ok := i.(reflect.Type); ok {
				if t != reflect.TypeOf(v) {
					err = c.NewError(errors.InvalidArgTypeError)
				}
			} else {
				if t, ok := i.(string); ok {
					if t != reflect.TypeOf(v).String() {
						err = c.NewError(errors.InvalidArgTypeError)
					}
				} else {
					if t, ok := i.(int); ok {
						switch t {
						case datatypes.IntType:
							_, ok = v.(int)

						case datatypes.FloatType:
							_, ok = v.(float64)

						case datatypes.BoolType:
							_, ok = v.(bool)

						case datatypes.StringType:
							_, ok = v.(string)

						default:
							ok = true
						}
						if !ok {
							err = c.NewError(errors.InvalidArgTypeError)
						}
					}
				}
			}
		} else {
			t := util.GetInt(i)
			switch t {
			case datatypes.ErrorType:
				v = errors.New(errors.Panic).Context(v)

			case datatypes.IntType:
				v = util.GetInt(v)

			case datatypes.FloatType:
				v = util.GetFloat(v)

			case datatypes.StringType:
				v = util.GetString(v)

			case datatypes.BoolType:
				v = util.GetBool(v)

			case datatypes.ArrayType:
				// If it's  not already an array, wrap it in one.
				if _, ok := v.([]interface{}); !ok {
					v = []interface{}{v}
				}

			case datatypes.StructType:
				// If it's not a struct, we can't do anything so fail
				if _, ok := v.(map[string]interface{}); !ok {
					return c.NewError(errors.InvalidTypeError)
				}

			case datatypes.UndefinedType, datatypes.InterfaceType, datatypes.ChanType:
				// No work at all to do here.

			default:
				return c.NewError(errors.InvalidTypeError)
			}
		}

		_ = c.Push(v)
	}

	return err
}

// CoerceImpl instruction processor.
func CoerceImpl(c *Context, i interface{}) *errors.EgoError {
	t := util.GetInt(i)

	v, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	switch t {
	case datatypes.ErrorType:
		v = errors.New(errors.Panic).Context(v)

	case datatypes.IntType:
		v = util.GetInt(v)

	case datatypes.FloatType:
		v = util.GetFloat(v)

	case datatypes.StringType:
		v = util.GetString(v)

	case datatypes.BoolType:
		v = util.GetBool(v)

	case datatypes.ArrayType:
		// If it's  not already an array, wrap it in one.
		if _, ok := v.(*datatypes.EgoArray); !ok {
			if _, ok := v.([]interface{}); !ok {
				array := datatypes.NewArray(datatypes.TypeOf(v), 1)
				_ = array.Set(0, v)
				v = array
			}
		}

	case datatypes.StructType:
		// If it's not a struct, we can't do anything so fail
		if _, ok := v.(map[string]interface{}); !ok {
			return c.NewError(errors.InvalidTypeError)
		}

	case datatypes.InterfaceType, datatypes.UndefinedType:
		// No work at all to do here.

	default:
		if t < datatypes.ArrayType {
			return c.NewError(errors.InvalidTypeError)
		}

		var base []interface{}

		if a, ok := v.(*datatypes.EgoArray); ok {
			base = a.BaseArray()
		} else {
			base = v.([]interface{})
		}

		elementType := t - datatypes.ArrayType
		array := datatypes.NewArray(elementType, len(base))
		model := datatypes.InstanceOf(elementType)

		for i, element := range base {
			_ = array.Set(i, util.Coerce(element, model))
		}

		v = array
	}

	_ = c.Push(v)

	return nil
}
