package time

import (
	"time"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// durationString implements the String() method for a duration. This has an extension
// beyond the standard Go function; if a boolean value is supplied then it indicates if
// the duration is to be formatted with spaces between the units. So "1h3m5s" vs "1h 3m 5s".
func durationString(s *symbols.SymbolTable, args data.List) (any, error) {
	var err error

	duration := getDuration(s)
	withSpaces := false

	if args.Len() > 0 {
		withSpaces, err = data.Bool(args.Get(0))
		if err != nil {
			return nil, errors.New(err).In("String")
		}
	}

	if duration != nil {
		return util.FormatDuration(*duration, withSpaces), nil
	}

	return nil, errors.ErrNoFunctionReceiver
}

// Helper function that retrieves the "this" value which is the receiver for a duration value.
func getDuration(s *symbols.SymbolTable) *time.Duration {
	if this, found := s.Get(defs.ThisVariable); found {
		if duration, err := data.GetNativeDuration(this); err == nil {
			return duration
		}
	}

	return nil
}

// parseDuration implements the time.ParseDuration function. It uses the extended parsing
// functions in the util package which allows for a duration string to include "d" unit for
// expressing days. The result is a (time.Duration, error) tuple.
func parseDuration(s *symbols.SymbolTable, args data.List) (any, error) {
	text := data.String(args.Get(0))

	duration, err := util.ParseDuration(text)

	return data.NewList(duration, err), err
}
