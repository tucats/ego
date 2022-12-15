package functions

import (
	"math"
	"math/rand"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Min implements the min() function.
func Min(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) == 1 {
		return args[0], nil
	}

	r := args[0]

	for _, v := range args[1:] {
		v = datatypes.Coerce(v, r)
		if v == nil {
			return nil, errors.New(errors.ErrInvalidType).In("min()")
		}

		switch rv := r.(type) {
		case byte, int32, int, int64:
			if datatypes.GetInt(v) < datatypes.GetInt(r) {
				r = v
			}

		case float32, float64:
			if datatypes.GetFloat64(v) < datatypes.GetFloat64(r) {
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
			return nil, errors.New(errors.ErrInvalidType).Context(datatypes.TypeOf(rv).String())
		}
	}

	return r, nil
}

// Max implements the max() function.
func Max(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) == 1 {
		return args[0], nil
	}

	r := args[0]

	for _, xv := range args[1:] {
		v := datatypes.Coerce(xv, r)
		if v == nil {
			return nil, errors.New(errors.ErrInvalidType).In("max()").Context(datatypes.TypeOf(r).String())
		}

		switch rr := r.(type) {
		case byte, int32, int, int64:
			if datatypes.GetInt(v) > datatypes.GetInt(r) {
				r = v
			}

		case float32, float64:
			if datatypes.GetFloat64(v) > datatypes.GetFloat64(r) {
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
			return nil, errors.New(errors.ErrInvalidType).In("max()").Context(datatypes.TypeOf(rr).String())
		}
	}

	return r, nil
}

// Sum implements the sum() function.
func Sum(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	base := args[0]

	for _, addendV := range args[1:] {
		addend := datatypes.Coerce(addendV, base)
		if addend == nil {
			return nil, errors.New(errors.ErrInvalidType).In("sum()").Context(datatypes.TypeOf(addendV).String())
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
			return nil, errors.New(errors.ErrInvalidType).In("sum()").Context(datatypes.TypeOf(rv).String())
		}
	}

	return base, nil
}

// Sqrt implements the sqrt() function.
func Sqrt(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	f := datatypes.GetFloat64(args[0])

	return math.Sqrt(f), nil
}

// Abs implements the abs() function.
func Abs(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	f := datatypes.GetFloat64(args[0])

	return math.Abs(f), nil
}

// Log is the log() function.
func Log(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	return math.Log(datatypes.GetFloat64(args[0])), nil
}

// Random implmeents the math.Random function.
func Random(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	max := datatypes.GetInt(args[0])
	if max <= 0 {
		return nil, errors.New(errors.ErrInvalidFunctionArgument).Context(max)
	}

	return rand.Intn(max), nil
}
