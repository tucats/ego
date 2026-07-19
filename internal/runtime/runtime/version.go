package runtime

import (
	"strconv"

	"github.com/araddon/dateparse"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

var VersionString string
var BuildTime string

func egoVersion(s *symbols.SymbolTable, args data.List) (any, error) {
	return "ego" + VersionString, nil
}

func egoBuildTime(s *symbols.SymbolTable, args data.List) (any, error) {
	t, err := dateparse.ParseAny(BuildTime)

	return data.NewList(t, err), err
}

// egoTypes implements the runtime.Types() function, which returns a string
// representation of the current active types mode in the running program
// or test. The types mode is set by configuration option, command-line
// option, or overridden in the program using the @type directive. A runtime
// error is triggered if there is an internal error accessing the type
// checking state value.
func egoTypes(s *symbols.SymbolTable, args data.List) (any, error) {
	const undefined = "undefined"

	types, found := s.GetAnyScope(defs.TypeCheckingVariable)
	if !found {
		return undefined, errors.ErrInternalCompiler.Context("__type_checking not found")
	}

	v, err := data.Int(types)
	if err != nil {
		return undefined, errors.ErrInternalCompiler.Context("__type_checking=" + data.String(types))
	}

	switch v {
	case 0:
		return "strict", nil

	case 1:
		return "relaxed", nil

	case 2:
		return "dynamic", nil

	default:
		return undefined, errors.ErrInternalCompiler.Context("__type_checking=" + strconv.Itoa(v))
	}
}
