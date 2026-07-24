package time

import (
	"time"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
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


// sleepUntil implements t.SleepUntil(). It is an Ego-specific convenience
// method with no direct Go equivalent -- Go code does the equivalent with
// time.Sleep(time.Until(t)). It pauses execution until the receiver's time
// value is reached; if that time has already passed, time.Until returns a
// non-positive duration and time.Sleep returns immediately, so this is a
// no-op for a time in the past rather than an error.
func sleepUntil(s *symbols.SymbolTable, args data.List) (any, error) {
	t, err := getTime(s)
	if err == nil {
		if d := time.Until(*t); d > 0 {
			time.Sleep(d)
		}
	}

	return err, err
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
