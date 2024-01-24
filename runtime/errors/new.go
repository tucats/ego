package errors

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func newError(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	if args.Len() < 1 || args.Len() > 2 {
		return nil, errors.ErrArgumentCount.In("New")
	}

	result := errors.Message(data.String(args.Get(0)))

	if args.Len() > 1 {
		_ = result.Context(args.Get(1))
	}

	if verbose {
		if module, found := s.Get(defs.ModuleVariable); found {
			_ = result.In(data.String(module))
		}

		if line, found := s.Get(defs.LineVariable); found {
			_ = result.At(data.Int(line), 0)
		}
	}

	return result, nil
}
