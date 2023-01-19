package bytecode

import (
	"reflect"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

// StaticTypeOpcode implements the StaticType opcode, which
// sets the static typing flag for the current context.
func staticTypingByteCode(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err == nil {
		if isStackMarker(v) {
			return c.error(errors.ErrFunctionReturnedVoid)
		}

		value := data.Int(v)
		if value < defs.StrictTypeEnforcement || value > defs.NoTypeEnforcement {
			return c.error(errors.ErrInvalidValue).Context(value)
		}

		c.typeStrictness = value
		c.symbols.SetAlways(defs.TypeCheckingVariable, value)
	}

	return err
}

func requiredTypeByteCode(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err == nil {
		if isStackMarker(v) {
			return c.error(errors.ErrFunctionReturnedVoid)
		}

		// If we're doing strict type checking...
		if c.typeStrictness == 0 {
			if t, ok := i.(reflect.Type); ok {
				if t != reflect.TypeOf(v) {
					err = c.error(errors.ErrArgumentType)
				}
			} else {
				if t, ok := i.(string); ok {
					if t != reflect.TypeOf(v).String() {
						err = c.error(errors.ErrArgumentType)
					}
				} else {
					if t, ok := i.(int); ok {
						switch data.TypeOf(t).Kind() {
						case data.IntKind:
							_, ok = v.(int)

						case data.Int32Kind:
							_, ok = v.(int32)

						case data.Int64Kind:
							_, ok = v.(int64)

						case data.ByteKind:
							_, ok = v.(byte)

						case data.BoolKind:
							_, ok = v.(bool)

						case data.StringKind:
							_, ok = v.(string)

						case data.Float32Kind:
							_, ok = v.(float32)

						case data.Float64Kind:
							_, ok = v.(float64)

						default:
							ok = true
						}

						if !ok {
							err = c.error(errors.ErrArgumentType)
						}
					}
				}
			}
		} else {
			t := data.TypeOf(i)
			// If it's not interface type, check it out...
			if !t.IsInterface() {
				if t.IsKind(data.ErrorKind) {
					v = errors.ErrPanic.Context(v)
				}

				// Figure out the type. If it's a user type, get the underlying type unless we're
				// testing against an interface (in which case we need the full type info to get the
				// list of functions).
				actualType := data.TypeOf(v)

				// *chan and chan will be considered valid matches
				if actualType.Kind() == data.PointerKind && actualType.BaseType().Kind() == data.ChanKind {
					actualType = actualType.BaseType()
				}

				if actualType.Kind() == data.TypeKind && !t.IsInterface() {
					actualType = actualType.BaseType()
				}

				if !actualType.IsType(t) {
					return c.error(errors.ErrArgumentType)
				}

				switch t.Kind() {
				case data.IntKind:
					v = data.Int(v)

				case data.Int32Kind:
					v = data.Int32(v)

				case data.Int64Kind:
					v = data.Int64(v)

				case data.BoolKind:
					v = data.Bool(v)

				case data.ByteKind:
					v = data.Byte(v)

				case data.Float32Kind:
					v = data.Float32(v)

				case data.Float64Kind:
					v = data.Float64(v)

				case data.StringKind:
					v = data.String(v)
				}
			} else {
				// It is an interface type, if it's a non-empty interface
				// verify the value against the interface entries.
				if t.HasFunctions() {
					vt := data.TypeOf(v)
					if e := t.ValidateFunctions(vt); e != nil {
						return c.error(e)
					}
				}
			}
		}

		_ = c.push(v)
	}

	return err
}

// coerceByteCode instruction processor.
func coerceByteCode(c *Context, i interface{}) error {
	t := data.TypeOf(i)

	v, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(v) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	// If we are in static mode, we don't do any coercions and require a match
	if c.typeStrictness == 0 {
		// If it's an interface we are converting to, no worries, it's a match and we're done.
		if t.IsInterface() {
			return c.push(v)
		}

		vt := data.TypeOf(v)
		if !vt.IsType(t) {
			return c.error(errors.ErrInvalidType).Context(vt.String())
		}

		return c.push(v)
	}

	// Some types cannot be coerced, so must match.
	if t.Kind() == data.MapKind ||
		t.Kind() == data.StructKind ||
		t.Kind() == data.ArrayKind {
		if !t.IsType(data.TypeOf(v)) {
			return c.error(errors.ErrInvalidType).Context(data.TypeOf(v).String())
		}
	}

	switch t.Kind() {
	case data.MapKind, data.ErrorKind, data.InterfaceKind, data.UndefinedKind:

	case data.StructKind:
		// Check all the fields in the struct to ensure they exist in the type.
		vv := v.(*data.Struct)
		for _, k := range vv.FieldNames() {
			_, e2 := t.Field(k)
			if e2 != nil {
				return errors.NewError(e2)
			}
		}

		// Verify that all the fields in the type are found in the object; if not,
		// create a zero-value for that type.
		for _, k := range t.FieldNames() {
			if _, found := vv.Get(k); !found {
				ft, _ := t.Field(k)
				vv.SetAlways(k, data.InstanceOfType(ft))
			}
		}

		v = vv

	case data.IntKind:
		v = data.Int(v)

	case data.Int32Kind:
		v = data.Int32(v)

	case data.Int64Kind:
		v = data.Int64(v)

	case data.BoolKind:
		v = data.Bool(v)

	case data.ByteKind:
		v = data.Byte(v)

	case data.Float32Kind:
		v = data.Float32(v)

	case data.Float64Kind:
		v = data.Float64(v)

	case data.StringKind:
		v = data.String(v)

	default:
		// If they are alread the same type, no work.
		if data.TypeOf(v).IsType(t) {
			return c.push(v)
		}

		var base []interface{}

		if a, ok := v.(*data.Array); ok {
			base = a.BaseArray()
		} else {
			base = v.([]interface{})
		}

		elementType := t.BaseType()
		array := data.NewArray(elementType, len(base))
		model := data.InstanceOfType(elementType)

		for i, element := range base {
			_ = array.Set(i, data.Coerce(element, model))
		}

		v = array
	}

	return c.push(v)
}

func (b ByteCode) NeedsCoerce(kind *data.Type) bool {
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
		return data.IsType(i.Operand, kind)
	}

	return true
}

func addressOfByteCode(c *Context, i interface{}) error {
	name := data.String(i)

	addr, ok := c.symbols.GetAddress(name)
	if !ok {
		return c.error(errors.ErrUnknownIdentifier).Context(name)
	}

	return c.push(addr)
}

func deRefByteCode(c *Context, i interface{}) error {
	name := data.String(i)

	addr, ok := c.symbols.GetAddress(name)
	if !ok {
		return c.error(errors.ErrUnknownIdentifier).Context(name)
	}

	if data.IsNil(addr) {
		return c.error(errors.ErrNilPointerReference)
	}

	if content, ok := addr.(*interface{}); ok {
		if data.IsNil(content) {
			return c.error(errors.ErrNilPointerReference)
		}

		c2 := *content
		if c3, ok := c2.(*interface{}); ok {
			if data.IsNil(content) {
				return c.error(errors.ErrNilPointerReference)
			}

			return c.push(*c3)
		}

		return c.error(errors.ErrNotAPointer).Context(data.Format(c2))
	}

	return c.error(errors.ErrNotAPointer).Context(name)
}
