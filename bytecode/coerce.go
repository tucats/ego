package bytecode

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

// coerceByteCode instruction processor.
func coerceByteCode(c *Context, i interface{}) error {
	var coerceOk bool

	t := data.TypeOf(i)

	v, err := c.PopWithoutUnwrapping()
	if err != nil {
		return err
	}

	if isStackMarker(v) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	if constant, ok := v.(data.Immutable); ok {
		v = c.unwrapConstant(constant)
		coerceOk = true
	}

	// If we are in static mode, we don't do any coercions and require a match
	if !coerceOk && c.typeStrictness == defs.StrictTypeEnforcement {
		// If it's an interface we are converting to, no worries, it's a match and we're done.
		if t.IsInterface() {
			return c.push(v)
		}

		// If they actually match, we're done.
		vt := data.TypeOf(v)
		if vt.IsType(t) {
			return c.push(v)
		}

		// If the type is "error" and this is a struct it need only support the Error() method to match the interfacd.
		if t.Name() == "error" {
			if pv, ok := v.(*interface{}); ok && pv != nil {
				vx := *pv
				if vv, ok := vx.(*data.Struct); ok {
					decl := vv.Type().GetFunctionDeclaration("Error")
					// Must have a functin "Error" with no parameteres and returns a single value which is a string.
					if decl != nil && decl.Name == "Error" && len(decl.Parameters) == 0 && len(decl.Returns) == 1 && decl.Returns[0].Kind() == data.StringKind {
						return c.push(v)
					}
				}

				if vv, ok := vx.(*data.Type); ok {
					decl := vv.GetFunctionDeclaration("Error")
					// Must have a functin "Error" with no parameteres and returns a single value which is a string.
					if decl != nil && decl.Name == "Error" && len(decl.Parameters) == 0 && len(decl.Returns) == 1 && decl.Returns[0].Kind() == data.StringKind {
						return c.push(v)
					}
				}
			}
		}

		// If they don't match, and one wasn't a constant (coerceOk), then
		// throw an error indicating this coercion is not allowed.
		return c.error(errors.ErrTypeMismatch).Context(vt.String() + ", " + t.String())
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
		for _, k := range vv.FieldNames(false) {
			_, e2 := t.Field(k)
			if e2 != nil {
				return errors.New(e2)
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
