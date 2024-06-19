package cipher

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

func sealString(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var err error

	arg := args.Get(0)

	if stringptr, ok := arg.(*string); ok {
		value := *stringptr
		seal := util.Seal(value)
		*stringptr = ""

		return string(seal), err
	}

	if stringptr, ok := arg.(*interface{}); ok {
		value := *stringptr
		if text, ok := value.(string); ok {
			seal := util.Seal(text)
			*stringptr = ""

			return string(seal), err
		}
	}

	return nil, errors.ErrInvalidPointerType.In("cipher.Seal").Context(data.TypeOf(arg).String())
}

func unsealString(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var err error

	text := data.String(args.Get(0))
	sealedString := util.NewSealedString(text)
	unsealed := sealedString.Unseal()

	return unsealed, err
}
