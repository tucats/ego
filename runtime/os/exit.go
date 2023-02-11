package os

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// exit implements the os.exit() function.
func exit(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	// If no arguments, just do a simple exit
	if args.Len() == 0 {
		return nil, errors.ErrExit.Context(0)
	}

	switch v := args.Get(0).(type) {
	case bool, byte, int32, int, int64, float32, float64:
		return nil, errors.ErrExit.Context(data.Int(args.Get(0)))

	case string:
		return nil, errors.ErrExit.Context(v)

	default:
		return nil, errors.ErrExit.Context(0)
	}
}
