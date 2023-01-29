package builtins

import (
	"math"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Length implements the len() function.
func Length(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
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

	case string:
		return len(arg), nil

	default:
		// Extensions have to be enabled and we must not be in strict
		// type checking mode to return length of the stringified argument.
		if v, found := s.Get(defs.ExtensionsVariable); found {
			if data.Bool(v) {
				if v, found := s.Get(defs.TypeCheckingVariable); found {
					if data.Int(v) > 0 {
						return len(data.String(arg)), nil
					}
				}
			}
		}

		// Otherwise, invalid type.
		return 0, errors.ErrArgumentType.In("len").Context(data.TypeOf(args[0]))
	}
}

// SizeOf returns the size in bytes of an arbibrary object.
func SizeOf(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	size := data.SizeOf(args[0])

	return size, nil
}
