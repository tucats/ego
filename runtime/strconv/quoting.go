package strconv

import (
	"strconv"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Quote implements the strconv.Quote() function.
func Quote(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	value := data.String(args[0])

	return strconv.Quote(value), nil
}

// Unquote implements the strconv.Unquote() function.
func Unquote(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	value := data.String(args[0])

	if v, err := strconv.Unquote(value); err != nil {
		return data.List(nil, err), errors.NewError(err).In("Unquote")
	} else {
		return data.List(v, nil), nil
	}
}
