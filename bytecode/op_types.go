package bytecode

import (
	"reflect"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// StaticTypeOpcode implements the StaticType opcode, which
// sets the static typing flag for the current context.
func staticTypingByteCode(c *Context, i interface{}) *errors.EgoError {
	v, err := c.Pop()
	if errors.Nil(err) {
		c.Static = util.GetBool(v)
		err = c.symbols.SetAlways("__static_data_types", c.Static)
	}

	return err
}

func requiredTypeByteCode(c *Context, i interface{}) *errors.EgoError {
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
						dataType := datatypes.TypeOf(t)
						if dataType.IsType(datatypes.IntType) {
							_, ok = v.(int)
						} else if dataType.IsType(datatypes.BoolType) {
							_, ok = v.(bool)
						} else if dataType.IsType(datatypes.StringType) {
							_, ok = v.(string)
						} else if dataType.IsType(datatypes.FloatType) {
							_, ok = v.(float64)
						} else {
							ok = true
						}

						if !ok {
							err = c.newError(errors.InvalidArgTypeError)
						}
					}
				}
			}
		} else {
			t := datatypes.GetType(i)

			if t.IsType(datatypes.ErrorType) {
				v = errors.New(errors.Panic).Context(v)
			} else if t.IsType(datatypes.IntType) {
				v = util.GetInt(v)
			} else if t.IsType(datatypes.FloatType) {
				v = util.GetFloat(v)
			} else if t.IsType(datatypes.StringType) {
				v = util.GetString(v)
			} else if t.IsType(datatypes.BoolType) {
				v = util.GetBool(v)
			} else if t.IsType(datatypes.PointerToType(datatypes.InterfaceType)) ||
				t.IsType(datatypes.PointerToType(datatypes.ChanType)) ||
				t.IsUndefined() ||
				t.IsType(datatypes.InterfaceType) ||
				t.IsType(datatypes.PointerToType(datatypes.InterfaceType)) ||
				t.IsType(datatypes.PointerToType(datatypes.ChanType)) ||
				t.IsType(datatypes.ChanType) {
				// No work to do here
			} else {
				return c.newError(errors.InvalidTypeError)
			}
		}

		_ = c.stackPush(v)
	}

	return err
}

// coerceByteCode instruction processor.
func coerceByteCode(c *Context, i interface{}) *errors.EgoError {
	t := datatypes.GetType(i)

	v, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	if t.IsType(datatypes.ErrorType) {

	} else if t.IsType(datatypes.IntType) {
		v = util.GetInt(v)
	} else if t.IsType(datatypes.FloatType) {
		v = util.GetFloat(v)
	} else if t.IsType(datatypes.BoolType) {
		v = util.GetBool(v)
	} else if t.IsType(datatypes.StringType) {
		v = util.GetString(v)
	} else if t.IsType(datatypes.InterfaceType) || t.IsUndefined() {
		// No work to do here.
	} else {
		var base []interface{}

		if a, ok := v.(*datatypes.EgoArray); ok {
			base = a.BaseArray()
		} else {
			base = v.([]interface{})
		}

		elementType := *t.ValueType
		array := datatypes.NewArray(elementType, len(base))
		model := datatypes.InstanceOfKind(elementType)

		for i, element := range base {
			_ = array.Set(i, util.Coerce(element, model))
		}

		v = array
	}

	_ = c.stackPush(v)

	return nil
}

func (b ByteCode) NeedsCoerce(kind datatypes.Type) bool {
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
		return datatypes.IsType(i.Operand, kind)
	}

	return true
}

func addressOfByteCode(c *Context, i interface{}) *errors.EgoError {
	name := util.GetString(i)

	addr, ok := c.symbols.GetAddress(name)
	if !ok {
		return c.newError(errors.UnknownIdentifierError).Context(name)
	}

	return c.stackPush(addr)
}

func derefByteCode(c *Context, i interface{}) *errors.EgoError {
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
