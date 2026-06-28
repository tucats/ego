package errors

import (
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// unwrap implements the (e error) unwrap() method for Ego errors.
func unwrap(s *symbols.SymbolTable, args data.List) (any, error) {
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
