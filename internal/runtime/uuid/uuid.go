package uuid

import (
	"github.com/google/uuid"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/util/strings"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
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
//
// This is declared with two return values (UUID, error), so the result must
// always be wrapped in a data.List -- see the "callRuntimeFunction dispatch
// mechanics" note in CLAUDE.md. Returning a bare (nil, error) here would be
// treated as a single-value result with a non-nil Go error, which aborts the
// program with an uncatchable runtime error instead of the documented
// catchable two-value return (BUG-40).
func parseUUID(symbols *symbols.SymbolTable, args data.List) (any, error) {
	s := data.String(args.Get(0))

	u, err := uuid.Parse(s)
	if err != nil {
		return data.NewList(nil, errors.New(err)), nil
	}

	result := data.NewStruct(UUIDTypeDef).SetNative(u)

	return data.NewList(result, nil), nil
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
