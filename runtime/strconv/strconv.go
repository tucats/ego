package strconv

import (
	"strconv"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// doAtoi implements the strconv.doAtoi() function.
func doAtoi(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	str := data.String(args[0])

	if v, err := strconv.Atoi(str); err != nil {
		return data.NewList(nil, err), errors.NewError(err).In("Atoi")
	} else {
		return data.NewList(v, nil), nil
	}
}

// doItoa implements the strconv.doItoa() function.
func doItoa(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	value := data.Int(args[0])

	return strconv.Itoa(value), nil
}
