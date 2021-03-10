package functions

import (
	"math"
	"math/rand"

	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// Min implements the min() function.
func Min(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) == 1 {
		return args[0], nil
	}

	r := args[0]

	for _, v := range args[1:] {
		v = util.Coerce(v, r)
		if v == nil {
			return nil, errors.New(errors.InvalidTypeError).In("min()")
		}

		switch r.(type) {
		case int:
			if v.(int) < r.(int) {
				r = v
			}

		case float64:
			if v.(float64) < r.(float64) {
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
			return nil, errors.New(errors.InvalidTypeError).In("min()")
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

	for _, v := range args[1:] {
		v = util.Coerce(v, r)
		if v == nil {
			return nil, errors.New(errors.InvalidTypeError).In("max()")
		}

		switch rr := r.(type) {
		case int:
			if v.(int) > rr {
				r = v
			}

		case float64:
			if v.(float64) > rr {
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
			return nil, errors.New(errors.InvalidTypeError).In("max()")
		}
	}

	return r, nil
}

// Sum implements the sum() function.
func Sum(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	base := args[0]

	for _, addend := range args[1:] {
		addend = util.Coerce(addend, base)
		if addend == nil {
			return nil, errors.New(errors.InvalidTypeError).In("sum()")
		}

		switch addend.(type) {
		case int:
			base = base.(int) + addend.(int)

		case float64:
			base = base.(float64) + addend.(float64)

		case string:
			base = base.(string) + addend.(string)

		case bool:
			base = base.(bool) || addend.(bool)

		default:
			return nil, errors.New(errors.InvalidTypeError).In("sum()")
		}
	}

	return base, nil
}

// Sqrt implements the sqrt() function.
func Sqrt(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	f := util.GetFloat(args[0])

	return math.Sqrt(f), nil
}

// Abs implements the abs() function.
func Abs(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	f := util.GetFloat(args[0])

	return math.Abs(f), nil
}

// Log is the log() function.
func Log(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	return math.Log(util.GetFloat(args[0])), nil
}

// Random implmeents the math.Random function.
func Random(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	max := util.GetInt(args[0])
	if max <= 0 {
		return nil, errors.New(errors.InvalidFunctionArgument).Context(max)
	}

	return rand.Intn(max), nil
}
