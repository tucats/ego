package json

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/jaxon"
	"github.com/tucats/ego/symbols"
)

// parse implements the json.Parse() function, which parses an arbitrary
// JSON string using a query expression parameter to extract a specific
// value from the JSON data.
func parse(s *symbols.SymbolTable, args data.List) (any, error) {
	text := data.String(args.Get(0))
	expression := data.String(args.Get(1))

	value, err := jaxon.GetItem(text, expression)
	if e, ok := err.(*jaxon.Error); ok {
		code, context := e.Extract()
		err = errors.Message(code).Context(context)
	}

	return value, err
}
