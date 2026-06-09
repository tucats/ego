package builtins

import (
	"fmt"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Length implements the len() function.
func Length(s *symbols.SymbolTable, args data.List) (any, error) {
	if args.Get(0) == nil {
		return 0, nil
	}

	switch arg := args.Get(0).(type) {
	// For a channel, return the number of items currently buffered.
	// BUILTIN-LENGTH-1 fix: the original code returned math.MaxInt32 for any
	// non-empty channel, which diverges from Go's len(channel) semantics and
	// misleads callers that use the result to check queue depth.  We now call
	// arg.Len() which returns the actual buffered item count (safe to call
	// concurrently — it delegates to Go's built-in len() on the channel).
	// A closed-and-drained channel returns 0 naturally via the same path.
	case *data.Channel:
		return arg.Len(), nil

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
			bv, err := data.Bool(v)
			if err != nil {
				return 0, err
			}

			if bv {
				if v, found := s.Get(defs.TypeCheckingVariable); found {
					tv, err := data.Int(v)
					if err != nil {
						return 0, err
					}

					if tv > 0 {
						return len(data.String(arg)), nil
					}
				}
			}
		}

		// Otherwise, invalid type.
		return 0, errors.ErrArgumentType.In("len").Context(fmt.Sprintf("argument %d: %s", 1, data.TypeOf(args.Get(0)).String()))
	}
}

// SizeOf returns the size in bytes of an arbitrary object.
func SizeOf(s *symbols.SymbolTable, args data.List) (any, error) {
	size := data.SizeOf(args.Get(0))

	return size, nil
}
