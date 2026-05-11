package json

import (
	stdjson "encoding/json"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/jaxon"
)

// parse implements the json.Parse() function, which parses an arbitrary
// JSON string using a query expression parameter to extract a specific
// value from the JSON data.
func parse(s *symbols.SymbolTable, args data.List) (any, error) {
	text := data.String(args.Get(0))
	expression := data.String(args.Get(1))

	value, err := jaxon.GetItem(text, expression)

	// Workaround for a jaxon limitation: GetItem fails with "element not found"
	// when the root expression "." is applied to a JSON array. Re-decode the input
	// and, if it is a valid array, return a compact JSON representation of it.
	if err != nil && expression == "." {
		var root any
		if decodeErr := stdjson.Unmarshal([]byte(text), &root); decodeErr == nil {
			if _, ok := root.([]any); ok {
				if encoded, encErr := stdjson.Marshal(root); encErr == nil {
					return data.NewList(string(encoded), nil), nil
				}
			}
		}
	}

	if e, ok := err.(*jaxon.Error); ok {
		code, context := e.Extract()
		err = errors.Message(code).Context(context)
	}

	return data.NewList(value, err), nil
}
