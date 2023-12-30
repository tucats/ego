package uuid

import (
	"github.com/google/uuid"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// newUUID implements the uuid.newUUID() function.
func newUUID(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	u := uuid.New()

	result := data.NewStruct(uuidTypeDef)
	err := result.Set("UUID", u)

	return result, err
}

// nilUUID implements the uuid.nilUUID() function.
func nilUUID(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	u := uuid.Nil

	result := data.NewStruct(uuidTypeDef)
	err := result.Set("UUID", u)

	return result, err
}

// parseUUID implements the uuid.parseUUID() function.
func parseUUID(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	s := data.String(args.Get(0))

	u, err := uuid.Parse(s)
	if err != nil {
		return nil, errors.NewError(err)
	}

	result := data.NewStruct(uuidTypeDef)
	err = result.Set("UUID", u)

	return result, err
}

// toString implements the (u uuid.UUID) String() function.
func toString(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	if v, found := s.Get(defs.ThisVariable); found {
		if UUID, ok := v.(*data.Struct); ok {
			if u, found := UUID.Get("UUID"); found {
				if actual, ok := u.(uuid.UUID); ok {
					return actual.String(), nil
				}
			}
		}
	}

	return nil, errors.ErrNoFunctionReceiver.In("String")
}

// toGibberish implements the (u uuid.UUID) Gibberish() function.
func toGibberish(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	if v, found := s.Get(defs.ThisVariable); found {
		if UUID, ok := v.(*data.Struct); ok {
			if u, found := UUID.Get("UUID"); found {
				if actual, ok := u.(uuid.UUID); ok {
					return util.Gibberish(actual), nil
				}
			}
		}
	}

	return nil, errors.ErrNoFunctionReceiver.In("String")
}
