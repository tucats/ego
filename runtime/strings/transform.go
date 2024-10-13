package strings

import (
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Wrapper around strings.join().
func join(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	elemArray, ok := args.Get(0).(*data.Array)
	if !ok {
		return nil, errors.ErrArgumentType.In("Join")
	}

	separator := data.String(args.Get(1))
	elements := make([]string, elemArray.Len())

	for i := 0; i < elemArray.Len(); i++ {
		element, _ := elemArray.Get(i)
		elements[i] = data.String(element)
	}

	return strings.Join(elements, separator), nil
}
