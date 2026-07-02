package builtins

import (
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// Close implements the generic close() function which can be used
// to close a channel or a file, or a database connection. Maybe later,
// other items as well.
//
// For a channel argument, Close() returns two values: a bool reporting
// whether the channel was open (and so is now newly closed), and an error
// that is non-nil if the channel was already closed. Returning the error
// as part of a data.List (via data.NewList below) is what makes it a
// normal, catchable Ego error — an Ego program can either capture it
// explicitly:
//
//	wasOpen, err := close(ch)
//
// or ignore the return values entirely and instead guard against a double
// close with try/catch:
//
//	try {
//	    close(ch)
//	} catch(e) {
//	    // e describes the "already closed" problem here
//	}
//
// See BUG-29 in docs/ISSUES.md: before this fix, closing an already-closed
// channel crashed the entire ego process instead of producing an error the
// Ego program could handle either way.
func Close(s *symbols.SymbolTable, args data.List) (any, error) {
	switch arg := args.Get(0).(type) {
	case *data.Channel:
		wasOpen, err := arg.Close()

		return data.NewList(wasOpen, err), nil

	case *data.Struct:
		// A struct means it's really a type. Store the actual object
		// as the "__this" value, and the try to call the "Close"
		// method associated with the type.
		s.SetAlways(defs.ThisVariable, arg)

		return callTypeMethod(arg.TypeString(), "Close", s, args)

	default:
		return nil, errors.ErrInvalidType.In("close")
	}
}
