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
// integer (rune) values, etc. It is called from within the Call
// bytecode when the target function is really a type.
func Cast(s *symbols.SymbolTable, args data.List) (any, error) {
	// Target t is the type of the last parameter
	t := data.TypeOf(args.Get(args.Len() - 1))

	// If there is a list of operand values, create an array from them to
	// use as the source.
	source := args.Get(0)
	if args.Len() > 2 {
		source = data.NewArrayFromList(data.InterfaceType, args.Slice(0, args.Len()-1))
	}

	if t.IsString() {
		// If the source is a []byte type, we can just fetch the bytes and do a direct conversion.
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
		// Casting a single character string enclosed in single quotes to a rune.
		return castToStringValue(t, actual, source)

	default:
		if t.IsArray() {
			r := data.NewArray(t.BaseType(), 1)

			value, err := data.Coerce(source, data.InstanceOfType(t.BaseType()))
			if err != nil {
				return nil, errors.New(err).In(t.BaseType().String())
			}

			_ = r.Set(0, value)

			return r, nil
		}

		v, err := data.Coerce(source, data.InstanceOfType(t))
		if err != nil {
			return nil, errors.New(err).In(t.String())
		}

		if v != nil {
			return v, nil
		}

		return nil, errors.ErrInvalidType.Context(data.TypeOf(source).String())
	}
}

func castToStringValue(t *data.Type, actual string, source any) (any, error) {
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

	return data.Coerce(source, data.InstanceOfType(t))
}

func castToArrayValue(t *data.Type, actual *data.Array) (any, error) {
	if t.IsType(actual.Type()) {
		return actual, nil
	}

	if t.IsString() && (actual.Type().IsIntegerType() || actual.Type().IsInterface()) {
		return convertIntArrayToString(actual)
	}

	elementKind := *t.BaseType()
	r := data.NewArray(t.BaseType(), actual.Len())

	for i := 0; i < actual.Len(); i++ {
		v, _ := actual.Get(i)

		switch elementKind.Kind() {
		case data.BoolKind:
			ev, err := data.Bool(v)
			if err != nil {
				return nil, err
			}

			_ = r.Set(i, ev)

		case data.ByteKind:
			ev, err := data.Byte(v)
			if err != nil {
				return nil, err
			}

			_ = r.Set(i, ev)

		case data.Int32Kind:
			ev, err := data.Int32(v)
			if err != nil {
				return nil, err
			}

			_ = r.Set(i, ev)

		case data.IntKind:
			ev, err := data.Int(v)
			if err != nil {
				return nil, err
			}

			_ = r.Set(i, ev)

		case data.Int64Kind:
			ev, err := data.Int64(v)
			if err != nil {
				return nil, err
			}

			_ = r.Set(i, ev)

		case data.Float32Kind:
			ev, err := data.Float32(v)
			if err != nil {
				return nil, err
			}

			_ = r.Set(i, ev)

		case data.Float64Kind:
			ev, err := data.Float64(v)
			if err != nil {
				return nil, err
			}

			_ = r.Set(i, ev)

		case data.StringKind:
			ev := data.String(v)
			_ = r.Set(i, ev)

		default:
			return nil, errors.ErrInvalidType.Context(data.TypeOf(v).String())
		}
	}

	return r, nil
}

func convertIntArrayToString(actual *data.Array) (any, error) {
	r := strings.Builder{}

	for i := 0; i < actual.Len(); i++ {
		ch, err := actual.Get(i)
		if err != nil {
			return nil, err
		}

		runeValue, err := data.Int32(ch)
		if err != nil {
			return nil, err
		}

		r.WriteRune(runeValue & math.MaxInt32)
	}

	return r.String(), nil
}

func castToString(source any) (any, error) {
	if actual, ok := source.(*data.Array); ok && actual != nil && actual.Type().IsType(data.ByteType) {
		b := actual.GetBytes()

		return string(b), nil
	} else if actual, ok := source.(*data.Array); ok && actual != nil && actual.Type().IsIntegerType() {
		r := strings.Builder{}

		for i := 0; i < actual.Len(); i++ {
			ch, _ := actual.Get(i)

			rv, err := data.Int32(ch)
			if err != nil {
				return nil, err
			}

			r.WriteRune(rune(rv))
		}

		return r.String(), nil
	} else {
		return data.FormatUnquoted(source), nil
	}
}
