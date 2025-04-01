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
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	if constant, ok := v.(data.Immutable); ok {
		v = c.unwrapConstant(constant)
		coerceOk = true
	}

	// If we are in static mode, we don't do any coercions and require a match
	if !coerceOk && c.typeStrictness == defs.StrictTypeEnforcement {
		return requireMatch(c, t, v)
	}

	// Some types cannot be coerced, so must match.
	if t.Kind() == data.MapKind ||
		t.Kind() == data.StructKind ||
		t.Kind() == data.ArrayKind {
		if !t.IsType(data.TypeOf(v)) {
			return c.runtimeError(errors.ErrInvalidType).Context(data.TypeOf(v).String())
		}
	}

	switch t.Kind() {
	case data.MapKind, data.ErrorKind, data.InterfaceKind, data.UndefinedKind:

	case data.StructKind:
		// Check all the fields in the struct to ensure they exist in the type.
		v, err = coerceStruct(v, t)
		if err != nil {
			return c.runtimeError(err)
		}

	case data.IntKind:
		v, err = data.Int(v)

	case data.Int32Kind:
		v, err = data.Int32(v)

	case data.Int64Kind:
		v, err = data.Int64(v)

	case data.BoolKind:
		v, err = data.Bool(v)

	case data.ByteKind:
		v, err = data.Byte(v)

	case data.Float32Kind:
		v, err = data.Float32(v)

	case data.Float64Kind:
		v, err = data.Float64(v)

	case data.StringKind:
		v = data.String(v)

	default:
		// If they are already the same type, no work.
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
			v, err := data.Coerce(element, model)
			if err != nil {
				return c.runtimeError(err)
			}

			_ = array.Set(i, v)
		}

		v = array
	}

	if err == nil {
		err = c.push(v)
	}

	return err
}

func coerceStruct(value interface{}, t *data.Type) (interface{}, error) {
	structValue := value.(*data.Struct)
	for _, fieldName := range structValue.FieldNames(false) {
		_, err := t.Field(fieldName)
		if err != nil {
			return nil, errors.New(err)
		}
	}

	// Verify that all the fields in the type are found in the object; if not,
	// create a zero-value for that type.
	for _, fieldName := range t.FieldNames() {
		if _, found := structValue.Get(fieldName); !found {
			fieldType, _ := t.Field(fieldName)
			structValue.SetAlways(fieldName, data.InstanceOfType(fieldType))
		}
	}

	return structValue, nil
}

// requireMatch checks if the specified value matches the type. The match must follow the
// strict type enforcement rules.
//
// If it's an interface we are converting to, it is considered successful.
// If the types match, it is considered successful.
// If the type is "error" and this is a struct and supports the error() method it is a match.
//
// Otherwise if they don't match, and one wasn't a constant (coerceOk), then
// throw an error indicating this coercion is not allowed.
func requireMatch(c *Context, t *data.Type, v interface{}) error {
	if t.IsInterface() {
		return c.push(v)
	}

	vt := data.TypeOf(v)
	if vt.IsType(t) {
		return c.push(v)
	}

	if t.Name() == "error" {
		if pv, ok := v.(*interface{}); ok && pv != nil {
			vx := *pv
			if vv, ok := vx.(*data.Struct); ok {
				decl := vv.Type().GetFunctionDeclaration("Error")

				if decl != nil && decl.Name == "Error" && len(decl.Parameters) == 0 && len(decl.Returns) == 1 && decl.Returns[0].Kind() == data.StringKind {
					return c.push(v)
				}
			}

			if vv, ok := vx.(*data.Type); ok {
				decl := vv.GetFunctionDeclaration("Error")

				if decl != nil && decl.Name == "Error" && len(decl.Parameters) == 0 && len(decl.Returns) == 1 && decl.Returns[0].Kind() == data.StringKind {
					return c.push(v)
				}
			}
		}
	}

	return c.runtimeError(errors.ErrTypeMismatch).Context(vt.String() + ", " + t.String())
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
