package i18n

import (
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/subs"
)

// Implement the i18n.Format() function.
func format(s *symbols.SymbolTable, args data.List) (any, error) {
	// Get the string to format.
	msg := data.String(args.Get(0))

	// If there is a second function argument, it is a map or struct
	// of the parameters used for the translation. The key value (or
	// field name) is the parameter name, and it's value is the parameter
	// value.
	parameters, err := constructParameterMap(args)
	if err != nil {
		err = errors.New(err)

		return data.NewList(nil, err), err
	}

	result, err := subs.SubstituteMap(msg, parameters)
	if err != nil {
		err = errors.New(err)

		return data.NewList(result, err), err
	}

	return data.NewList(result, nil), nil
}
