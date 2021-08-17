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
					err = c.newError(errors.ErrInvalidArgType)
				}
			} else {
				if t, ok := i.(string); ok {
					if t != reflect.TypeOf(v).String() {
						err = c.newError(errors.ErrInvalidArgType)
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
						} else if dataType.IsType(datatypes.Float32Type) {
							_, ok = v.(float32)
						} else if dataType.IsType(datatypes.FloatType) {
							_, ok = v.(float64)
						} else {
							ok = true
						}

						if !ok {
							err = c.newError(errors.ErrInvalidArgType)
						}
					}
				}
			}
		} else {
			t := datatypes.GetType(i)

			// If it's not interface type, check it out...
			if !t.IsType(datatypes.InterfaceType) {
				if t.IsType(datatypes.ErrorType) {
					v = errors.New(errors.ErrPanic).Context(v)
				}

				actualType := datatypes.TypeOf(v)
				if actualType.Kind() == datatypes.TypeKind {
					actualType = *actualType.BaseType()
				}

				if !actualType.IsType(t) {
					return c.newError(errors.ErrArgumentType)
				}

				if t.IsType(datatypes.IntType) {
					v = util.GetInt(v)
				} else if t.IsType(datatypes.FloatType) {
					v = util.GetFloat64(v)
				} else if t.IsType(datatypes.StringType) {
					v = util.GetString(v)
				} else if t.IsType(datatypes.BoolType) {
					v = util.GetBool(v)
				}
			}
		}

		_ = c.stackPush(v)
	}

	return err
}

// coerceByteCode instruction processor.
func coerceByteCode(c *Context, i interface{}) *errors.EgoError {
	// If we are in static mode, we don't do any coercions.
	if c.Static {
		return nil
	}

	t := datatypes.GetType(i)

	v, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	// Some types cannot be coerced, so must match.
	if t.Kind() == datatypes.MapKind ||
		t.Kind() == datatypes.StructKind ||
		t.Kind() == datatypes.ArrayKind {
		if !t.IsType(datatypes.TypeOf(v)) {
			return c.newError(errors.ErrInvalidType)
		}
	}

	// @tomcole restructure this back as a switch statement based on Kind()
	if t.Kind() == datatypes.MapKind {

	} else if t.Kind() == datatypes.StructKind {
		// Check all the fields in the struct to ensure they exist in the type.
		vv := v.(*datatypes.EgoStruct)
		for _, k := range vv.FieldNames() {
			_, e2 := t.Field(k)
			if !errors.Nil(e2) {
				return e2
			}
		}

		// Verify that all the fields in the type are found in the object; if not,
		// create a zero-value for that type.
		for _, k := range t.FieldNames() {
			if _, found := vv.Get(k); !found {
				ft, _ := t.Field(k)
				vv.SetAlways(k, datatypes.InstanceOfType(ft))
			}
		}

		v = vv
	} else if t.IsType(datatypes.ErrorType) {

	} else if t.IsType(datatypes.IntType) {
		v = util.GetInt(v)
	} else if t.IsType(datatypes.FloatType) {
		v = util.GetFloat64(v)
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

		elementType := *t.BaseType()
		array := datatypes.NewArray(elementType, len(base))
		model := datatypes.InstanceOfType(elementType)

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
		return c.newError(errors.ErrUnknownIdentifier).Context(name)
	}

	return c.stackPush(addr)
}

func deRefByteCode(c *Context, i interface{}) *errors.EgoError {
	name := util.GetString(i)

	addr, ok := c.symbols.GetAddress(name)
	if !ok {
		return c.newError(errors.ErrUnknownIdentifier).Context(name)
	}

	if datatypes.IsNil(addr) {
		return c.newError(errors.ErrNilPointerReference)
	}

	if content, ok := addr.(*interface{}); ok {
		if datatypes.IsNil(content) {
			return c.newError(errors.ErrNilPointerReference)
		}

		c2 := *content
		if c3, ok := c2.(*interface{}); ok {
			if datatypes.IsNil(content) {
				return c.newError(errors.ErrNilPointerReference)
			}

			return c.stackPush(*c3)
		}

		return c.newError(errors.ErrNotAPointer).Context(datatypes.Format(c2))
	}

	return c.newError(errors.ErrNotAPointer).Context(name)
}
