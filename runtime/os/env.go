package os

import (
	"os"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Getenv implements the os.Getenv() function which reads
// an environment variable from the os.
func Getenv(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return os.Getenv(data.String(args[0])), nil
}

// Clearenv implements the os.Clearenv() function.
func Clearenv(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	os.Clearenv()

	return nil, nil
}

// Environ implements the os.Environ() function.
func Environ(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	env := os.Environ()

	result := data.NewArray(data.StringType, len(env))

	for index, value := range env {
		if err := result.Set(index, value); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// Executable implements the os.Executable() function.
func Executable(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	path, err := os.Executable()

	if err != nil {
		err = errors.NewError(err)
	}

	return path, err
}

// Args implements os.Args() which fetches command-line arguments from
// the Ego command invocation, if any.
func Args(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	r, found := s.Get(defs.CLIArgumentListVariable)
	if !found {
		r = data.NewArray(data.StringType, 0)
	}

	return r, nil
}
