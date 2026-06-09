package builtins

import (
	"reflect"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Index implements the index() function.
func Index(symbols *symbols.SymbolTable, args data.List) (any, error) {
	if !extensions() {
		return nil, errors.ErrExtension.Context("index")
	}

	switch arg := args.Get(0).(type) {
	case *data.Array:
		for i := 0; i < arg.Len(); i++ {
			vv, _ := arg.Get(i)
			if reflect.DeepEqual(vv, args.Get(1)) {
				return i, nil
			}
		}

		return -1, nil

	case []any:
		return nil, errors.ErrInvalidType.Context("[]any")

	case *data.Map:
		// BUILTIN-INDEX-1 fix: the previous code returned the raw bool from
		// arg.Get, making index(map, key) return a bool while all other cases
		// return an int.  The function's declaration says it returns int, and
		// callers that use the result arithmetically would receive a bool.
		// We now return 1 (found) or 0 (not found) to match the array and
		// string cases, and we still propagate any map-lookup error.
		_, found, err := arg.Get(args.Get(1))
		if found {
			return 1, err
		}

		return 0, err

	default:
		v := data.String(args.Get(0))
		p := data.String(args.Get(1))

		return strings.Index(v, p) + 1, nil
	}
}
