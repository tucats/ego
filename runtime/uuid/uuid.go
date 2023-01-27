package uuid

import (
	"github.com/google/uuid"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// New implements the uuid.New() function.
func New(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	u := uuid.New()

	result := data.NewStruct(uuidTypeDef)
	err := result.Set("UUID", u)

	return result, err
}

// Nil implements the uuid.Nil() function.
func Nil(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	u := uuid.Nil

	result := data.NewStruct(uuidTypeDef)
	err := result.Set("UUID", u)

	return result, err
}

// Parse implements the uuid.Parse() function.
func Parse(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	s := data.String(args[0])

	u, err := uuid.Parse(s)
	if err != nil {
		return nil, errors.NewError(err)
	}

	result := data.NewStruct(uuidTypeDef)
	err = result.Set("UUID", u)

	return result, err
}

// String implements the (u uuid.UUID) String() function.
func String(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if v, found := s.Get(defs.ThisVariable); found {
		if UUID, ok := v.(*data.Struct); ok {
			if u, found := UUID.Get("UUID"); found {
				if actual, ok := u.(uuid.UUID); ok {
					return actual.String(), nil
				}
			}
		}
	}

	return nil, errors.ErrNoFunctionReceiver.In("String()")
}
