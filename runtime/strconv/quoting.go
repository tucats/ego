package strconv

import (
	"strconv"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// doQuote implements the strconv.doQuote() function.
func doQuote(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	value := data.String(args.Get(0))

	return strconv.Quote(value), nil
}

// doUnquote implements the strconv.doUnquote() function.
func doUnquote(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	value := data.String(args.Get(0))

	if v, err := strconv.Unquote(value); err != nil {
		return data.NewList(nil, err), errors.New(err).In("Unquote")
	} else {
		return data.NewList(v, nil), nil
	}
}
