package strconv

import (
	"strconv"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Atoi implements the strconv.Atoi() function.
func Atoi(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	str := data.String(args[0])

	if v, err := strconv.Atoi(str); err != nil {
		return data.List(nil, err), errors.NewError(err).Context("Atoi")
	} else {
		return data.List(v, nil), nil
	}
}

// Itoa implements the strconv.Itoa() function.
func Itoa(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	value := data.Int(args[0])

	return strconv.Itoa(value), nil
}
