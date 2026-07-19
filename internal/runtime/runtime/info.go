package runtime

import (
	"fmt"
	"strconv"

	"github.com/araddon/dateparse"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

var VersionString string
var BuildTime string

// egoVersion implements the runtime.Ego() string which returns the
// Ego version string information. This is distinct from runtime.Version()
// which returns the Go version string on which the Ego program was
// built.
func egoVersion(s *symbols.SymbolTable, args data.List) (any, error) {
	return "ego" + VersionString, nil
}

// buildTime implements the runtime.BuildTime() function, which returns
// the *time.Time value of the timestamp when the Ego program itself was
// last built.
func buildTime(s *symbols.SymbolTable, args data.List) (any, error) {
	t, err := dateparse.ParseAny(BuildTime)

	return data.NewList(t, err), err
}

// mode implements the runtime.Mode() function, which returns
// the current execution mode string. If there is no mode string
// set, this means REPL interactive mode. Other possible values
// expected are "test", "run", and "server".
func mode(s *symbols.SymbolTable, args data.List) (any, error) {
	modeValue, found := s.GetAnyScope(defs.ModeVariable)
	if !found {
		return "interactive", nil
	}

	if data.String(modeValue) == "" {
		return "", errors.ErrInternalRuntime.Context(fmt.Sprintf("%s empty", defs.ModeVariable))
	}

	return modeValue, nil
}

// types implements the runtime.Types() function, which returns a string
// representation of the current active types mode in the running program
// or test. The types mode is set by configuration option, command-line
// option, or overridden in the program using the @type directive. A runtime
// error is triggered if there is an internal error accessing the type
// checking state value.
func types(s *symbols.SymbolTable, args data.List) (any, error) {
	const undefined = "undefined"

	types, found := s.GetAnyScope(defs.TypeCheckingVariable)
	if !found {
		return undefined, errors.ErrInternalRuntime.Context("__type_checking not found")
	}

	v, err := data.Int(types)
	if err != nil {
		return undefined, errors.ErrInternalRuntime.Context("__type_checking=" + data.String(types))
	}

	switch v {
	case 0:
		return "strict", nil

	case 1:
		return "relaxed", nil

	case 2:
		return "dynamic", nil

	default:
		return undefined, errors.ErrInternalRuntime.Context("__type_checking=" + strconv.Itoa(v))
	}
}
