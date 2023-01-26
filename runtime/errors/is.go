package errors

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Is implements the (e error) Is() method for Ego errors.
func Is(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 0 {
		return nil, errors.ErrArgumentCount.In("Error()")
	}

	var test error

	if e, ok := args[0].(*errors.Error); ok {
		test = e
	}

	if v, found := s.Get(defs.ThisVariable); found {
		if e, ok := v.(*errors.Error); ok {
			return e.Is(test), nil
		}

		return nil, errors.ErrInvalidType.Context(data.TypeOf(v)).In("Is()")
	}

	return nil, errors.ErrNoFunctionReceiver
}
