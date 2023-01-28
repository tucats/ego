package errors

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func newError(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, errors.ErrArgumentCount.In("New()")
	}

	result := errors.NewMessage(data.String(args[0]))

	if len(args) > 1 {
		_ = result.Context(args[1])
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
