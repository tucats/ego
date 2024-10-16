package time

import (
	"time"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

func durationString(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	duration := getDuration(s)
	withSpaces := false

	if args.Len() > 0 {
		withSpaces = data.Bool(args.Get(0))
	}

	if duration != nil {
		return util.FormatDuration(*duration, withSpaces), nil
	}

	return nil, errors.ErrNoFunctionReceiver
}

func getDuration(s *symbols.SymbolTable) *time.Duration {
	if this, found := s.Get(defs.ThisVariable); found {
		if duration, err := data.GetNativeDuration(this); err == nil {
			return duration
		}
	}

	return nil
}
