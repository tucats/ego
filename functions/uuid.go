package functions

import (
	"github.com/google/uuid"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// UUIDNew implements the uuid.New() function.
func UUIDNew(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	u := uuid.New()

	return u.String(), nil
}

// UUIDNil implements the uuid.Nil() function.
func UUIDNil(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	u := uuid.Nil

	return u.String(), nil
}

// UUIDParse implements the uuid.Parse() function.
func UUIDParse(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	s := data.String(args[0])

	u, err := uuid.Parse(s)
	if err != nil {
		return nil, errors.NewError(err)
	}

	return u.String(), nil
}

func Hostname(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return util.Hostname(), nil
}
