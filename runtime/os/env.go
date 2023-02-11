package os

import (
	"os"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// getEnv implements the os.getEnv() function which reads
// an environment variable from the os.
func getEnv(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	return os.Getenv(data.String(args.Get(0))), nil
}

// clearEnv implements the os.clearEnv() function.
func clearEnv(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	os.Clearenv()

	return nil, nil
}

// Environ implements the os.Environ() function.
func Environ(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	env := os.Environ()

	result := data.NewArray(data.StringType, len(env))

	for index, value := range env {
		if err := result.Set(index, value); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// executable implements the os.executable() function.
func executable(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	path, err := os.Executable()

	if err != nil {
		err = errors.NewError(err).In("Executable")
	}

	return path, err
}

// args implements os.args() which fetches command-line arguments from
// the Ego command invocation, if any.
func args(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	r, found := s.Get(defs.CLIArgumentListVariable)
	if !found {
		r = data.NewArray(data.StringType, 0)
	}

	return r, nil
}
