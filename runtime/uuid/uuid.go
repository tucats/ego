package uuid

import (
	"github.com/google/uuid"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// newUUID implements the uuid.newUUID() function.
func newUUID(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	return data.NewStruct(uuidTypeDef).SetNative(uuid.New()), nil
}

// nilUUID implements the uuid.nilUUID() function.
func nilUUID(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	return data.NewStruct(uuidTypeDef).SetNative(uuid.Nil), nil
}

// parseUUID implements the uuid.parseUUID() function.
func parseUUID(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	s := data.String(args.Get(0))

	u, err := uuid.Parse(s)
	if err != nil {
		return nil, errors.New(err)
	}

	result := data.NewStruct(uuidTypeDef).SetNative(u)

	return result, err
}

// toString implements the (u uuid.UUID) String() function.
func toString(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	if v, found := s.Get(defs.ThisVariable); found {
		if u, err := data.GetNativeUUID(v); err == nil {
			return u.String(), nil
		} else {
			return "", err
		}
	}

	return nil, errors.ErrNoFunctionReceiver.In("String")
}

// toGibberish implements the (u uuid.UUID) Gibberish() function.
func toGibberish(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	if v, found := s.Get(defs.ThisVariable); found {
		if u, err := data.GetNativeUUID(v); err == nil {
			return egostrings.Gibberish(u), nil
		} else {
			return "", err
		}
	}

	return nil, errors.ErrNoFunctionReceiver.In("Gibberish")
}
