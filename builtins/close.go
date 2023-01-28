package builtins

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Close implements the generic close() function which can be used
// to close a channel or a file, or a database connection. Maybe later,
// other items as well.
func Close(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	switch arg := args[0].(type) {
	case *data.Channel:
		return arg.Close(), nil

	case *data.Struct:
		// A struct means it's really a type. Store the actual object
		// as the "__this" value, and the try to call the "Close"
		// method associated with the type.
		s.SetAlways(defs.ThisVariable, arg)

		return callTypeMethod(arg.TypeString(), "Close", s, args)

	default:
		return nil, errors.ErrInvalidType.In("close()")
	}
}
