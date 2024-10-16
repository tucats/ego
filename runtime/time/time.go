package time

import (
	"time"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// String implements t.String(). This must be exported to be visible to the
// runtime formatting package for use with the "%v" format verb.
func String(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	t, err := getTime(s)
	if err != nil {
		return nil, err
	}

	layout := basicLayout

	return t.Format(layout), nil
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
