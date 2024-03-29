package strings

import (
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// toLower implements the lower() function.
func toLower(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	return strings.ToLower(data.String(args.Get(0))), nil
}

// toUpper implements the upper() function.
func toUpper(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	return strings.ToUpper(data.String(args.Get(0))), nil
}

// splitString splits a string into lines separated by a newline. Optionally
// a different delimiter can be supplied as the second argument.
func splitString(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var (
		v     []string
		delim = "\n"
	)

	src := data.String(args.Get(0))

	if args.Len() > 1 {
		delim = data.String(args.Get(1))
	}

	// Are we seeing Windows-style line endings? If we are doing a split
	// based on line endings, use Windows line endings.
	if delim == "\n" && strings.Index(src, "\r\n") > 0 {
		v = strings.Split(src, "\r\n")
	} else {
		// Otherwise, split by the delimiter
		v = strings.Split(src, delim)
	}

	// We need to store the result in a native Ego array.
	r := data.NewArray(data.StringType, len(v))

	for i, n := range v {
		if err := r.Set(i, n); err != nil {
			return nil, errors.New(err)
		}
	}

	return r, nil
}

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
