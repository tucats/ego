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
					err = c.newError(errors.InvalidArgTypeError)
				}
			} else {
				if t, ok := i.(string); ok {
					if t != reflect.TypeOf(v).String() {
						err = c.newError(errors.InvalidArgTypeError)
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
							err = c.newError(errors.InvalidArgTypeError)
						}
					}
				}
			}
		} else {
			t := util.GetInt(i)
			// If it's a pointer type, we can't do coercions
			if t > datatypes.PointerType {
				actualT := datatypes.PointerTo(v) + datatypes.PointerType
				if t != actualT /* && actualT != datatypes.InterfaceType  */ {
					return c.newError(errors.ArgumentTypeError)
				}
			} else {
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
						return c.newError(errors.InvalidTypeError)
					}

				case datatypes.UndefinedType, datatypes.InterfaceType, datatypes.ChanType:
					// No work at all to do here.

				default:
					return c.newError(errors.InvalidTypeError)
				}
			}
		}

		_ = c.stackPush(v)
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
			return c.newError(errors.InvalidTypeError)
		}

	case datatypes.InterfaceType, datatypes.UndefinedType:
		// No work at all to do here.

	default:
		if t < datatypes.ArrayType {
			return c.newError(errors.InvalidTypeError)
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

	_ = c.stackPush(v)

	return nil
}

func (b ByteCode) NeedsCoerce(kind int) bool {
	// If there are no instructions before this, no coerce is appropriate.
	pos := b.Mark()
	if pos == 0 {
		return false
	}

	i := b.GetInstruction(pos - 1)
	if i == nil {
		return false
	}

	if i.Operation == Push {
		return datatypes.TypeOf(i.Operand) != kind
	}

	return true
}

func AddressOfImpl(c *Context, i interface{}) *errors.EgoError {
	name := util.GetString(i)

	addr, ok := c.symbols.GetAddress(name)
	if !ok {
		return c.newError(errors.UnknownIdentifierError).Context(name)
	}

	return c.stackPush(addr)
}

func DeRefImpl(c *Context, i interface{}) *errors.EgoError {
	name := util.GetString(i)

	addr, ok := c.symbols.GetAddress(name)
	if !ok {
		return c.newError(errors.UnknownIdentifierError).Context(name)
	}

	if datatypes.IsNil(addr) {
		return c.newError(errors.NilPointerReferenceError)
	}

	if content, ok := addr.(*interface{}); ok {
		if datatypes.IsNil(content) {
			return c.newError(errors.NilPointerReferenceError)
		}

		c2 := *content
		if c3, ok := c2.(*interface{}); ok {
			if datatypes.IsNil(content) {
				return c.newError(errors.NilPointerReferenceError)
			}

			return c.stackPush(*c3)
		} else {
			return c.stackPush(c2)
		}
	}

	return c.newError(errors.NotAPointer).Context(name)
}
