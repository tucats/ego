package time

import (
	"time"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// String implements t.String(). This must be exported to be visible to the
// runtime formatting package for use with the "%v" format verb.
func String(s *symbols.SymbolTable, args data.List) (any, error) {
	t, err := getTime(s)
	if err != nil {
		return nil, err
	}

	return t.Format(time.UnixDate), nil
}

// getTime looks in the symbol table for the "this" receiver, and
// extracts the time value from it.
func getTime(symbols *symbols.SymbolTable) (*time.Time, error) {
	if t, ok := symbols.Get(defs.ThisVariable); ok {
		if timeValue, ok := t.(*time.Time); ok {
			return timeValue, nil
		}

		if timeValue, ok := t.(time.Time); ok {
			return &timeValue, nil
		}

		return data.GetNativeTime(t)
	}

	return nil, errors.ErrNoFunctionReceiver.In("time function")
}
