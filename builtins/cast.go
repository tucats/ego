package builtins

import (
	"math"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Compiler-generate casting. This is used to convert numeric arrays
// to a different kind of array, to convert a string to an array of
// integer (rune) values, etc.  It is called from within the Call
// bytecode when the target function is really a type.
func Cast(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	// Target t is the type of the last parameter
	t := data.TypeOf(args.Get(args.Len() - 1))

	// If there is a list of operand values, create an array from them to
	// use as the source.
	source := args.Get(0)
	if args.Len() > 2 {
		source = data.NewArrayFromList(data.InterfaceType, args.Slice(0, args.Len()-1))
	}

	if t.IsString() {
		// If the source is a []byte type, we can just fetch the bytes and do a direct convesion.
		// If the source is a []int type, we can convert each integer to a rune and add it to a
		// string builder. Otherwise, just format it as a string value.
		return castToString(source)
	}

	// If the target type is an interface type, construct a wrapper around
	// the value.
	if t.Kind() == data.InterfaceKind {
		return data.Wrap(source), nil
	}

	switch actual := source.(type) {
	// Conversion of one array type to another
	case *data.Array:
		return castToArrayValue(t, actual)

	case string:
		// Casting a single character string enclosed in single qutoes to a rune.
		return castToStringValue(t, actual, source)

	default:
		if t.IsArray() {
			r := data.NewArray(t.BaseType(), 1)
			value := data.Coerce(source, data.InstanceOfType(t.BaseType()))
			_ = r.Set(0, value)

			return r, nil
		}

		v := data.Coerce(source, data.InstanceOfType(t))
		if v != nil {
			return v, nil
		}

		return nil, errors.ErrInvalidType.Context(data.TypeOf(source).String())
	}
}

func castToStringValue(t *data.Type, actual string, source interface{}) (interface{}, error) {
	if t.IsType(data.Int32Type) {
		if len(actual) == 3 && actual[0] == '\'' && actual[2] == '\'' {
			return int32(actual[1]), nil
		}
	}

	if t.IsType(data.ArrayType(data.IntType)) {
		r := data.NewArray(data.IntType, 0)

		for _, rune := range actual {
			r.Append(int(rune))
		}

		return r, nil
	}

	if t.IsType(data.ArrayType(data.ByteType)) {
		r := data.NewArray(data.ByteType, 0)

		for i := 0; i < len(actual); i++ {
			r.Append(actual[i])
		}

		return r, nil
	}

	return data.Coerce(source, data.InstanceOfType(t)), nil
}

func castToArrayValue(t *data.Type, actual *data.Array) (interface{}, error) {
	if t.IsType(actual.Type()) {
		return actual, nil
	}

	if t.IsString() && (actual.Type().IsIntegerType() || actual.Type().IsInterface()) {
		r := strings.Builder{}

		for i := 0; i < actual.Len(); i++ {
			ch, _ := actual.Get(i)
			r.WriteRune(data.Int32(ch) & math.MaxInt32)
		}

		return r.String(), nil
	}

	elementKind := *t.BaseType()
	r := data.NewArray(t.BaseType(), actual.Len())

	for i := 0; i < actual.Len(); i++ {
		v, _ := actual.Get(i)

		switch elementKind.Kind() {
		case data.BoolKind:
			_ = r.Set(i, data.Bool(v))

		case data.ByteKind:
			_ = r.Set(i, data.Byte(v))

		case data.Int32Kind:
			_ = r.Set(i, data.Int32(v))

		case data.IntKind:
			_ = r.Set(i, data.Int(v))

		case data.Int64Kind:
			_ = r.Set(i, data.Int64(v))

		case data.Float32Kind:
			_ = r.Set(i, data.Float32(v))

		case data.Float64Kind:
			_ = r.Set(i, data.Float64(v))

		case data.StringKind:
			_ = r.Set(i, data.String(v))

		default:
			return nil, errors.ErrInvalidType.Context(data.TypeOf(v).String())
		}
	}

	return r, nil
}

func castToString(source interface{}) (interface{}, error) {
	if actual, ok := source.(*data.Array); ok && actual != nil && actual.Type().IsType(data.ByteType) {
		b := actual.GetBytes()

		return string(b), nil
	} else if actual, ok := source.(*data.Array); ok && actual != nil && actual.Type().IsIntegerType() {
		r := strings.Builder{}

		for i := 0; i < actual.Len(); i++ {
			ch, _ := actual.Get(i)
			r.WriteRune(rune(data.Int(ch) & math.MaxInt32))
		}

		return r.String(), nil
	} else {
		return data.FormatUnquoted(source), nil
	}
}
