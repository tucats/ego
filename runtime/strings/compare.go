package strings

import (
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

// Wrapper around strings.compare().
func compare(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	a := data.String(args.Get(0))
	b := data.String(args.Get(1))

	return strings.Compare(a, b), nil
}

// Wrapper around strings.equalFold().
func equalFold(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	a := data.String(args.Get(0))
	b := data.String(args.Get(1))

	return strings.EqualFold(a, b), nil
}
