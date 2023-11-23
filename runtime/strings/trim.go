package strings

import (
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

// trimSpace implements the strings.index() function.
func trimSpace(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	return strings.TrimSpace(data.String(args.Get(0))), nil
}

// trimPrefix implements the strings.TrimPrefix function.
func trimPrefix(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	return strings.TrimPrefix(data.String(args.Get(0)), data.String(args.Get(1))), nil
}

// trimSuffix implements the strings.TrimSuffix function.
func trimSuffix(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	return strings.TrimSuffix(data.String(args.Get(0)), data.String(args.Get(1))), nil
}
