package bytecode

import (
	"reflect"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
)

// StaticTypeOpcode implements the StaticType opcode, which
// sets the static typing flag for the current context.
func staticTypingByteCode(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err == nil {
		if IsStackMarker(v) {
			return c.newError(errors.ErrFunctionReturnedVoid)
		}

		c.Static = datatypes.Bool(v)
		c.symbols.SetAlways("__static_data_types", c.Static)
	}

	return err
}

func requiredTypeByteCode(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err == nil {
		if IsStackMarker(v) {
			return c.newError(errors.ErrFunctionReturnedVoid)
		}

		// If we're doing strict type checking...
		if c.Static {
			if t, ok := i.(reflect.Type); ok {
				if t != reflect.TypeOf(v) {
					err = c.newError(errors.ErrArgumentType)
				}
			} else {
				if t, ok := i.(string); ok {
					if t != reflect.TypeOf(v).String() {
						err = c.newError(errors.ErrArgumentType)
					}
				} else {
					if t, ok := i.(int); ok {
						switch datatypes.TypeOf(t).Kind() {
						case datatypes.IntKind:
							_, ok = v.(int)

						case datatypes.Int32Kind:
							_, ok = v.(int32)

						case datatypes.Int64Kind:
							_, ok = v.(int64)

						case datatypes.ByteKind:
							_, ok = v.(byte)

						case datatypes.BoolKind:
							_, ok = v.(bool)

						case datatypes.StringKind:
							_, ok = v.(string)

						case datatypes.Float32Kind:
							_, ok = v.(float32)

						case datatypes.Float64Kind:
							_, ok = v.(float64)

						default:
							ok = true
						}

						if !ok {
							err = c.newError(errors.ErrArgumentType)
						}
					}
				}
			}
		} else {
			t := datatypes.TypeOf(i)
			// If it's not interface type, check it out...
			if !t.IsInterface() {
				if t.IsKind(datatypes.ErrorKind) {
					v = errors.EgoError(errors.ErrPanic).Context(v)
				}

				// Figure out the type. If it's a user type, get the underlying type unless we're
				// testing against an interface (in which case we need the full type info to get the
				// list of functions).
				actualType := datatypes.TypeOf(v)

				// *chan and chan will be considered valid matches
				if actualType.Kind() == datatypes.PointerKind && actualType.BaseType().Kind() == datatypes.ChanKind {
					actualType = actualType.BaseType()
				}

				if actualType.Kind() == datatypes.TypeKind && !t.IsInterface() {
					actualType = actualType.BaseType()
				}

				if !actualType.IsType(t) {
					return c.newError(errors.ErrArgumentType)
				}

				switch t.Kind() {
				case datatypes.IntKind:
					v = datatypes.Int(v)

				case datatypes.Int32Kind:
					v = datatypes.Int32(v)

				case datatypes.Int64Kind:
					v = datatypes.Int64(v)

				case datatypes.BoolKind:
					v = datatypes.Bool(v)

				case datatypes.ByteKind:
					v = datatypes.Byte(v)

				case datatypes.Float32Kind:
					v = datatypes.Float32(v)

				case datatypes.Float64Kind:
					v = datatypes.Float64(v)

				case datatypes.StringKind:
					v = datatypes.String(v)
				}
			} else {
				// It is an interface type, if it's a non-empty interface
				// verify the value against the interface entries.
				if t.HasFunctions() {
					vt := datatypes.TypeOf(v)
					if e := t.ValidateFunctions(vt); e != nil {
						return c.newError(e)
					}
				}
			}
		}

		_ = c.stackPush(v)
	}

	return err
}

// coerceByteCode instruction processor.
func coerceByteCode(c *Context, i interface{}) error {
	// If we are in static mode, we don't do any coercions.
	if c.Static {
		return nil
	}

	t := datatypes.TypeOf(i)

	v, err := c.Pop()
	if err != nil {
		return err
	}

	if IsStackMarker(v) {
		return c.newError(errors.ErrFunctionReturnedVoid)
	}

	// Some types cannot be coerced, so must match.
	if t.Kind() == datatypes.MapKind ||
		t.Kind() == datatypes.StructKind ||
		t.Kind() == datatypes.ArrayKind {
		if !t.IsType(datatypes.TypeOf(v)) {
			return c.newError(errors.ErrInvalidType).Context(datatypes.TypeOf(v).String())
		}
	}

	switch t.Kind() {
	case datatypes.MapKind, datatypes.ErrorKind, datatypes.InterfaceKind, datatypes.UndefinedKind:

	case datatypes.StructKind:
		// Check all the fields in the struct to ensure they exist in the type.
		vv := v.(*datatypes.EgoStruct)
		for _, k := range vv.FieldNames() {
			_, e2 := t.Field(k)
			if e2 != nil {
				return errors.EgoError(e2)
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

	case datatypes.IntKind:
		v = datatypes.Int(v)

	case datatypes.Int32Kind:
		v = datatypes.Int32(v)

	case datatypes.Int64Kind:
		v = datatypes.Int64(v)

	case datatypes.BoolKind:
		v = datatypes.Bool(v)

	case datatypes.ByteKind:
		v = datatypes.Byte(v)

	case datatypes.Float32Kind:
		v = datatypes.Float32(v)

	case datatypes.Float64Kind:
		v = datatypes.Float64(v)

	case datatypes.StringKind:
		v = datatypes.String(v)

	default:
		// If they are alread the same type, no work.
		if datatypes.TypeOf(v).IsType(t) {
			return c.stackPush(v)
		}

		var base []interface{}

		if a, ok := v.(*datatypes.EgoArray); ok {
			base = a.BaseArray()
		} else {
			base = v.([]interface{})
		}

		elementType := t.BaseType()
		array := datatypes.NewArray(elementType, len(base))
		model := datatypes.InstanceOfType(elementType)

		for i, element := range base {
			_ = array.Set(i, datatypes.Coerce(element, model))
		}

		v = array
	}

	_ = c.stackPush(v)

	return nil
}

func (b ByteCode) NeedsCoerce(kind *datatypes.Type) bool {
	// If there are no instructions before this, no coerce is appropriate.
	pos := b.Mark()
	if pos == 0 {
		return false
	}

	i := b.Instruction(pos - 1)
	if i == nil {
		return false
	}

	if i.Operation == Push {
		return datatypes.IsType(i.Operand, kind)
	}

	return true
}

func addressOfByteCode(c *Context, i interface{}) error {
	name := datatypes.String(i)

	addr, ok := c.symbols.GetAddress(name)
	if !ok {
		return c.newError(errors.ErrUnknownIdentifier).Context(name)
	}

	return c.stackPush(addr)
}

func deRefByteCode(c *Context, i interface{}) error {
	name := datatypes.String(i)

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
