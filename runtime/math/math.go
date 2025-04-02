package math

import (
	"math/rand"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// normalize coerces a value to match the type of a model value. The
// (possibly modified) value is returned as the function value.
func normalize(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	v1, v2, err := data.Normalize(args.Get(0), args.Get(1))

	return data.NewList(v1, v2), err
}

// minimum implements the math.Min() function.
func minimum(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	var err error

	if args.Len() == 1 {
		return args.Get(0), nil
	}

	r := args.Get(0)

	for _, v := range args.Elements()[1:] {
		v, err = data.Coerce(v, r)
		if err != nil {
			return nil, errors.New(err).In("Min")
		}

		switch rv := r.(type) {
		case byte, int32, int, int64:
			if data.Int64OrZero(v) < data.Int64OrZero(r) {
				r = v
			}

		case float32, float64:
			if data.Float64OrZero(v) < data.Float64OrZero(r) {
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
func maximum(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	if args.Len() == 1 {
		return args.Get(0), nil
	}

	r := args.Get(0)

	for _, xv := range args.Elements()[1:] {
		v, err := data.Coerce(xv, r)
		if err != nil {
			return nil, errors.New(err).In("Max")
		}

		switch rr := r.(type) {
		case byte, int32, int, int64:
			if data.Int64OrZero(v) > data.Int64OrZero(r) {
				r = v
			}

		case float32, float64:
			if data.Float64OrZero(v) > data.Float64OrZero(r) {
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
func sum(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	base := args.Get(0)

	for _, addendV := range args.Elements()[1:] {
		addend, err := data.Coerce(addendV, base)
		if err != nil {
			return nil, errors.New(err).In("Sum")
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

// random implements the math.Random() function.
func random(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	maxValue, err := data.Int(args.Get(0))
	if err != nil {
		return nil, errors.New(err).In("math.Random")
	}

	if maxValue <= 0 {
		return nil, errors.ErrInvalidFunctionArgument.Context(maxValue)
	}

	return rand.Intn(maxValue), nil
}
