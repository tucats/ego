package errors

import (
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// isError implements the (e error) Is() method for Ego errors.
func isError(s *symbols.SymbolTable, args data.List) (any, error) {
	var test error

	if e, ok := args.Get(0).(*errors.Error); ok {
		test = e
	}

	if v, found := s.Get(defs.ThisVariable); found {
		if e, ok := v.(*errors.Error); ok {
			return e.Is(test), nil
		}

		return nil, errors.ErrInvalidType.Context(data.TypeOf(v)).In("Is")
	}

	return nil, errors.ErrNoFunctionReceiver
}
