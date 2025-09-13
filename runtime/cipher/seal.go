package cipher

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

func sealString(s *symbols.SymbolTable, args data.List) (any, error) {
	var err error

	arg := args.Get(0)

	if stringPointer, ok := arg.(*string); ok {
		value := *stringPointer
		seal := util.Seal(value)
		*stringPointer = ""

		return string(seal), err
	}

	if stringPointer, ok := arg.(*any); ok {
		value := *stringPointer
		if text, ok := value.(string); ok {
			seal := util.Seal(text)
			*stringPointer = ""

			return string(seal), err
		}
	}

	return nil, errors.ErrInvalidPointerType.In("cipher.Seal").Context(data.TypeOf(arg).String())
}

func unsealString(s *symbols.SymbolTable, args data.List) (any, error) {
	var err error

	text := data.String(args.Get(0))
	sealedString := util.NewSealedString(text)
	unsealed := sealedString.Unseal()

	return unsealed, err
}
