package errors

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// isError implements the (e error) Is() method for Ego errors.
func isError(s *symbols.SymbolTable, args data.List) (interface{}, error) {
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
