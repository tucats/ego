package errors

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Unwrap implements the (e error) Unwrap() method for Ego errors.
func Unwrap(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.ErrArgumentCount.In("Error()")
	}

	if v, found := s.Get(defs.ThisVariable); found {
		if e, ok := v.(*errors.Error); ok {
			return e.GetContext(), nil
		}

		if _, ok := v.(error); ok {
			return nil, nil
		}

		return nil, errors.ErrInvalidType.Context(data.TypeOf(v))
	}

	return nil, errors.ErrNoFunctionReceiver
}
