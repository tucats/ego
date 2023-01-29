package builtins

import (
	"math"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Length implements the len() function.
func Length(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return 0, nil
	}

	switch arg := args[0].(type) {
	// For a channel, it's length either zero if it's drained, or bottomless
	case *data.Channel:
		size := int(math.MaxInt32)
		if arg.IsEmpty() {
			size = 0
		}

		return size, nil

	case *data.Array:
		return arg.Len(), nil

	case error:
		return len(arg.Error()), nil

	case *data.Map:
		return len(arg.Keys()), nil

	case *data.Package:
		return nil, errors.ErrInvalidType.Context(data.TypeOf(arg).String())

	case nil:
		return 0, nil

	default:
		v := data.Coerce(args[0], "")
		if v == nil {
			return 0, nil
		}

		return len(v.(string)), nil
	}
}

// SizeOf returns the size in bytes of an arbibrary object.
func SizeOf(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	size := data.SizeOf(args[0])

	return size, nil
}
