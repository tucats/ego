package errors

import (
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

func newError(s *symbols.SymbolTable, args data.List) (any, error) {
	result := errors.Message(data.String(args.Get(0))).SetUser(true)

	if args.Len() > 1 {
		context := args.Get(1)
		result = result.Context(context)
	}

	if verbose {
		if module, found := s.Get(defs.ModuleVariable); found {
			result = result.In(data.String(module))
		}

		if line, found := s.Get(defs.LineVariable); found {
			result = result.At(data.IntOrZero(line), 0)
		}
	}

	return result, nil
}
