package strings

import (
	"strings"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// Split implements the Ego strings.Split() function. It exists here as a shim because
// it makes the second argument optional. According to the documentation, if the second
// argument is not provided, the function should split on newlines.
func split(s *symbols.SymbolTable, args data.List) (any, error) {
	text := data.String(args.Get(0))
	separator := "\n"

	if args.Len() > 1 {
		separator = data.String(args.Get(1))
	}

	a := strings.Split(text, separator)

	return data.NewArrayFromStrings(a...), nil
}
