package bytecode

import (
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// coerceByteCode instruction processor.
func coerceByteCode(c *Context, i any) error {
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
	// -- except for a numeric constant, which is allowed to adapt to the
	// declared return type the same way it does for assignment, expressions,
	// and function arguments, but only losslessly (BUG-68).
	if c.typeStrictness == defs.StrictTypeEnforcement {
		if !coerceOk {
			return requireMatch(c, t, v)
		}

		if data.IsNumeric(v) && data.IsNumeric(t) {
			coerced, err := data.CoerceLossless(v, data.InstanceOfType(t))
			if err != nil {
				return c.runtimeError(err)
			}

			return c.push(coerced)
		}
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

	case data.Int8Kind:
		v, err = data.Int8(v)

	case data.Int16Kind:
		v, err = data.Int16(v)

	case data.UInt16Kind:
		v, err = data.UInt16(v)

	case data.IntKind:
		v, err = data.Int(v)

	case data.Int32Kind:
		v, err = data.Int32(v)

	case data.Int64Kind:
		v, err = data.Int64(v)

	case data.UInt32Kind:
		v, err = data.UInt32(v)

	case data.UInt64Kind:
		v, err = data.UInt64(v)

	case data.UIntKind:
		v, err = data.UInt(v)

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
		// If the value is nil, no coercion needed.
		if v == nil {
			return c.push(v)
		}

		// If they are already the same type, no work.
		if data.TypeOf(v).IsType(t) {
			return c.push(v)
		}

		var base []any

		if a, ok := v.(*data.Array); ok {
			base = a.BaseArray()
		} else {
			if b, ok := v.([]any); ok {
				base = b
			} else {
				return c.push(v)
			}
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

func coerceStruct(value any, t *data.Type) (any, error) {
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

// isNillableKind reports whether nil is a valid value of the given type -- that
// is, whether nil is that type's zero value. In Go (and Ego) these are the
// pointer, function, map, slice, channel, and interface types; the built-in
// "error" type is interface-like and included as well.
func isNillableKind(t *data.Type) bool {
	switch t.Kind() {
	case data.PointerKind,
		data.FunctionKind,
		data.MapKind,
		data.ArrayKind,
		data.ChanKind,
		data.InterfaceKind,
		data.ErrorKind:
		return true

	default:
		return false
	}
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
func requireMatch(c *Context, t *data.Type, v any) error {
	if t.IsInterface() {
		return c.push(v)
	}

	// nil is the zero value for every nillable type, so it must be accepted
	// against any of them. Go allows nil for pointers, functions, maps,
	// slices, channels, and interfaces; the built-in "error" type is
	// interface-like and gets the same treatment (its Kind is data.ErrorKind,
	// not data.InterfaceKind, so the IsInterface() check above does not catch
	// it). Without the FunctionKind and ChanKind entries here, "return nil"
	// from a function declared to return a func or a chan failed in strict
	// mode with a spurious type mismatch even though nil is exactly that
	// type's zero value.
	if v == nil && isNillableKind(t) {
		return c.push(v)
	}

	vt := data.TypeOf(v)
	if vt.IsType(t) {
		return c.push(v)
	}

	if t.Name() == "error" {
		if pv, ok := v.(*any); ok && pv != nil {
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

// NeedsCoerce reports whether the compiler should emit a Coerce instruction
// for the given target type after whatever was just compiled.
//
// The decision is based on the most recently emitted instruction:
//
//   - If it was a Push whose operand is ALREADY the target type, the value is
//     already correct and no Coerce is needed → return false.
//   - If it was a Push whose operand is a DIFFERENT type, the runtime will
//     need to convert it → return true (COERCE-1 fix: previously this returned
//     false, meaning the Coerce was silently skipped for non-matching Push
//     operands such as int literals in a []float64 array).
//   - If the last instruction was anything other than Push (a computed value
//     whose type is not statically known), conservatively emit Coerce → true.
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
		// Coerce is only needed when the pushed value does NOT already match
		// the target type.  If it already matches, emitting Coerce would be a
		// redundant no-op at runtime.
		return !data.IsType(i.Operand, kind)
	}

	return true
}
