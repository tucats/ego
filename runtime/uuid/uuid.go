package uuid

import (
	"github.com/google/uuid"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// newUUID implements the uuid.New() function.
func newUUID(symbols *symbols.SymbolTable, args data.List) (any, error) {
	return data.NewStruct(UUIDTypeDef).SetNative(uuid.New()), nil
}

// nilUUID implements the uuid.nilUUID() function.
func nilUUID(symbols *symbols.SymbolTable, args data.List) (any, error) {
	return data.NewStruct(UUIDTypeDef).SetNative(uuid.Nil), nil
}

// parseUUID implements the uuid.parseUUID() function.
func parseUUID(symbols *symbols.SymbolTable, args data.List) (any, error) {
	s := data.String(args.Get(0))

	u, err := uuid.Parse(s)
	if err != nil {
		return nil, errors.New(err)
	}

	result := data.NewStruct(UUIDTypeDef).SetNative(u)

	return result, err
}

// toString implements the (u uuid.UUID) String() function.
func toString(s *symbols.SymbolTable, args data.List) (any, error) {
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
func toGibberish(s *symbols.SymbolTable, args data.List) (any, error) {
	if v, found := s.Get(defs.ThisVariable); found {
		if u, err := data.GetNativeUUID(v); err == nil {
			return egostrings.Gibberish(u), nil
		} else {
			return "", err
		}
	}

	return nil, errors.ErrNoFunctionReceiver.In("Gibberish")
}
