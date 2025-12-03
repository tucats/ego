package i18n

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
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
		return nil, err
	}

	m := map[string]any{}
	for k, v := range parameters {
		m[k] = v
	}

	formatted := subs.SubstituteMap(msg, m)

	return formatted, nil
}
