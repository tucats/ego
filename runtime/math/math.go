package math

import (
	"math"
	"math/rand"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// normalize coerces a value to match the type of a model value. The
// (possibly modified) value is returned as the function value.
func normalize(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	v1, v2 := data.Normalize(args[0], args[1])

	return data.NewList(v1, v2), nil
}

// minimum implements the math.Min() function.
func minimum(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) == 1 {
		return args[0], nil
	}

	r := args[0]

	for _, v := range args[1:] {
		v = data.Coerce(v, r)
		if v == nil {
			return nil, errors.ErrInvalidType.In("Min")
		}

		switch rv := r.(type) {
		case byte, int32, int, int64:
			if data.Int(v) < data.Int(r) {
				r = v
			}

		case float32, float64:
			if data.Float64(v) < data.Float64(r) {
				r = v
			}

		case string:
			if v.(string) < r.(string) {
				r = v
			}

		case bool:
			if !v.(bool) {
				r = v
			}
		default:
			return nil, errors.ErrInvalidType.Context(data.TypeOf(rv).String())
		}
	}

	return r, nil
}

// maximum implements the math.Max() function.
func maximum(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) == 1 {
		return args[0], nil
	}

	r := args[0]

	for _, xv := range args[1:] {
		v := data.Coerce(xv, r)
		if v == nil {
			return nil, errors.ErrInvalidType.In("Max").Context(data.TypeOf(r).String())
		}

		switch rr := r.(type) {
		case byte, int32, int, int64:
			if data.Int(v) > data.Int(r) {
				r = v
			}

		case float32, float64:
			if data.Float64(v) > data.Float64(r) {
				r = v
			}

		case string:
			if v.(string) > rr {
				r = v
			}

		case bool:
			if v.(bool) {
				r = v
			}

		default:
			return nil, errors.ErrInvalidType.In("Max").Context(data.TypeOf(rr).String())
		}
	}

	return r, nil
}

// sum implements the math.Sum() function.
func sum(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	base := args[0]

	for _, addendV := range args[1:] {
		addend := data.Coerce(addendV, base)
		if addend == nil {
			return nil, errors.ErrInvalidType.In("Sum").Context(data.TypeOf(addendV).String())
		}

		switch rv := addend.(type) {
		case bool:
			base = base.(bool) || addend.(bool)

		case byte:
			base = base.(byte) + addend.(byte)

		case int32:
			base = base.(int32) + addend.(int32)

		case int:
			base = base.(int) + addend.(int)

		case int64:
			base = base.(int) + addend.(int)

		case float32:
			base = base.(float32) + addend.(float32)

		case float64:
			base = base.(float64) + addend.(float64)

		case string:
			base = base.(string) + addend.(string)

		default:
			return nil, errors.ErrInvalidType.In("Sum").Context(data.TypeOf(rv).String())
		}
	}

	return base, nil
}

// squareRoot implements the math.Sqrt() function.
func squareRoot(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	f := data.Float64(args[0])

	return math.Sqrt(f), nil
}

// abs implements the math.Abs() function.
func abs(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	f := data.Float64(args[0])

	return math.Abs(f), nil
}

// log implements the math.Log() function.
func log(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return math.Log(data.Float64(args[0])), nil
}

// random implmeents the math.Random() function.
func random(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	max := data.Int(args[0])
	if max <= 0 {
		return nil, errors.ErrInvalidFunctionArgument.Context(max)
	}

	return rand.Intn(max), nil
}
