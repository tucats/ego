package time

import (
	"github.com/araddon/dateparse"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func Parse(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	value := data.String(args.Get(0))

	t, e := dateparse.ParseAny(value)
	if e != nil {
		e = errors.New(e).In("ParseAny")

		return data.NewList(nil, e), e
	}

	return data.NewList(t, nil), nil
}
