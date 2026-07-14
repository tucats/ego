package cipher

import (
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/util"
)

func sealString(s *symbols.SymbolTable, args data.List) (any, error) {
	arg := args.Get(0)

	if stringPointer, ok := arg.(*string); ok {
		value := *stringPointer
		seal := util.Seal(value)
		*stringPointer = ""

		return data.NewList(string(seal), nil), nil
	}

	if stringPointer, ok := arg.(*any); ok {
		if text, ok := (*stringPointer).(string); ok {
			seal := util.Seal(text)
			*stringPointer = ""

			return data.NewList(string(seal), nil), nil
		}
	}

	err := errors.ErrInvalidPointerType.In("cipher.Seal").Context(data.TypeOf(arg).String())

	return data.NewList(nil, err), err
}

func unsealString(s *symbols.SymbolTable, args data.List) (any, error) {
	text := data.String(args.Get(0))
	sealedString := util.NewSealedString(text)

	return sealedString.Unseal(), nil
}
