package os

import (
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// exit implements the os.exit() function.
func exit(symbols *symbols.SymbolTable, args data.List) (any, error) {
	// If no arguments, just do a simple exit
	if args.Len() == 0 {
		return nil, errors.ErrExit.Context(0)
	}

	switch v := args.Get(0).(type) {
	case bool, byte, int32, int, int64, float32, float64:
		return nil, errors.ErrExit.Context(data.IntOrZero(args.Get(0)))

	case string:
		return nil, errors.ErrExit.Context(v)

	default:
		return nil, errors.ErrExit.Context(0)
	}
}
