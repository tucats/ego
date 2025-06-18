package time

import (
	"github.com/araddon/dateparse"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Parse an arbitrary string value into a native Go datetime value. Uses the dateparse
// package which first scans the string to determine the appropriate Go date format string,
// and then uses that string to do the conversion.
func Parse(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	value := data.String(args.Get(0))

	t, e := dateparse.ParseAny(value)
	if e != nil {
		e = errors.New(e).In("ParseAny")

		return data.NewList(nil, e), e
	}

	return data.NewList(t, nil), nil
}
