package time

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

// format implements time.format().
func format(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	t, err := getTime(s)
	if err != nil {
		return nil, err
	}

	layout := data.String(args.Get(0))

	return t.Format(layout), nil
}

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
