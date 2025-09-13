package errors

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// context implements the (e error) Context() method for Ego errors.
func context(s *symbols.SymbolTable, args data.List) (any, error) {
	value := args.Get(0)

	if v, found := s.Get(defs.ThisVariable); found {
		if e, ok := v.(*errors.Error); ok {
			return e.Context(value), nil
		}

		return nil, errors.ErrInvalidType.Context(data.TypeOf(v)).In("Context")
	}

	return nil, errors.ErrNoFunctionReceiver
}

// at implements the (e error) At() method for Ego errors.
func at(s *symbols.SymbolTable, args data.List) (any, error) {
	value, err := data.Int(args.Get(0))
	if err != nil {
		return nil, errors.New(err).In("At")
	}

	if v, found := s.Get(defs.ThisVariable); found {
		if e, ok := v.(*errors.Error); ok {
			return e.At(value, 0), nil
		}

		return nil, errors.ErrInvalidType.Context(data.TypeOf(v)).In("At")
	}

	return nil, errors.ErrNoFunctionReceiver
}

// in implements the (e error) In() method for Ego errors.
func in(s *symbols.SymbolTable, args data.List) (any, error) {
	value := data.String(args.Get(0))

	if v, found := s.Get(defs.ThisVariable); found {
		if e, ok := v.(*errors.Error); ok {
			return e.In(value), nil
		}

		return nil, errors.ErrInvalidType.Context(data.TypeOf(v)).In("In")
	}

	return nil, errors.ErrNoFunctionReceiver
}
