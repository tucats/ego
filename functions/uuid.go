package functions

import (
	"github.com/google/uuid"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// UUIDNew implements the uuid.New() function.
func UUIDNew(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	u := uuid.New()

	return u.String(), nil
}

// UUIDNil implements the uuid.Nil() function.
func UUIDNil(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	u := uuid.Nil

	return u.String(), nil
}

// UUIDParse implements the uuid.Parse() function.
func UUIDParse(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	s := util.GetString(args[0])

	u, err := uuid.Parse(s)
	if err != nil {
		return nil, errors.New(err)
	}

	return u.String(), nil
}

func Hostname(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	return util.Hostname(), nil
}
