package strings

import (
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

// replace implements the strings.Replace() function.
func replace(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	return strings.Replace(
		data.String(args.Get(0)),
		data.String(args.Get(1)),
		data.String(args.Get(2)),
		data.Int(args.Get(3))), nil
}

// replaceAll implements the strings.ReplaceAll() function.
func replaceAll(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	return strings.ReplaceAll(
		data.String(args.Get(0)),
		data.String(args.Get(1)),
		data.String(args.Get(2))), nil
}
