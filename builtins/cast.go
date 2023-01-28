package builtins

import (
	"math"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Compiler-generate casting; generally always array types. This is used to
// convert numeric arrays to a different kind of array, to convert a string
// to an array of integer (rune) values, etc.  It is called from within
// the Call bytecode when the function is really a type.
func Cast(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	// Target kind is the last parameter
	kind := data.TypeOf(args[len(args)-1])

	source := args[0]
	if len(args) > 2 {
		source = data.NewArrayFromArray(data.InterfaceType, args[:len(args)-1])
	}

	if kind.IsKind(data.StringKind) {
		r := strings.Builder{}

		// If the source is an array of integers, treat them as runes to re-assemble.
		if actual, ok := source.(*data.Array); ok && actual != nil && actual.ValueType().IsIntegerType() {
			for i := 0; i < actual.Len(); i++ {
				ch, _ := actual.Get(i)
				r.WriteRune(rune(data.Int(ch) & math.MaxInt32))
			}
		} else {
			str := data.FormatUnquoted(source)
			r.WriteString(str)
		}

		return r.String(), nil
	}

	switch actual := source.(type) {
	// Conversion of one array type to another
	case *data.Array:
		if kind.IsType(actual.ValueType()) {
			return actual, nil
		}

		if kind.IsKind(data.StringKind) &&
			(actual.ValueType().IsIntegerType() || actual.ValueType().IsInterface()) {
			r := strings.Builder{}

			for i := 0; i < actual.Len(); i++ {
				ch, _ := actual.Get(i)
				r.WriteRune(data.Int32(ch) & math.MaxInt32)
			}

			return r.String(), nil
		}

		elementKind := *kind.BaseType()
		r := data.NewArray(kind.BaseType(), actual.Len())

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

	case string:
		if kind.IsType(data.ArrayType(data.IntType)) {
			r := data.NewArray(data.IntType, 0)

			for _, rune := range actual {
				r.Append(int(rune))
			}

			return r, nil
		}

		if kind.IsType(data.ArrayType(data.ByteType)) {
			r := data.NewArray(data.ByteType, 0)

			for _, rune := range actual {
				r.Append(int(rune))
			}

			return r, nil
		}

		return data.Coerce(source, data.InstanceOfType(kind)), nil

	default:
		if kind.IsArray() {
			r := data.NewArray(kind.BaseType(), 1)
			value := data.Coerce(source, data.InstanceOfType(kind.BaseType()))
			_ = r.Set(0, value)

			return r, nil
		}

		v := data.Coerce(source, data.InstanceOfType(kind))
		if v != nil {
			return v, nil
		}

		return nil, errors.ErrInvalidType.Context(data.TypeOf(source).String())
	}
}
