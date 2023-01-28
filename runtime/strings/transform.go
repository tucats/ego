package strings

import (
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// ToLower implements the lower() function.
func ToLower(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return strings.ToLower(data.String(args[0])), nil
}

// ToUpper implements the upper() function.
func ToUpper(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return strings.ToUpper(data.String(args[0])), nil
}

// Split splits a string into lines separated by a newline. Optionally
// a different delimiter can be supplied as the second argument.
func Split(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var v []string

	src := data.String(args[0])
	delim := "\n"

	if len(args) > 1 {
		delim = data.String(args[1])
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
		err := r.Set(i, n)
		if err != nil {
			return nil, errors.NewError(err)
		}
	}

	return r, nil
}

// Wrapper around strings.Join().
func Join(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	elemArray, ok := args[0].(*data.Array)
	if !ok {
		return nil, errors.ErrArgumentType.Context("Join()")
	}

	separator := data.String(args[1])
	elements := make([]string, elemArray.Len())

	for i := 0; i < elemArray.Len(); i++ {
		element, _ := elemArray.Get(i)
		elements[i] = data.String(element)
	}

	return strings.Join(elements, separator), nil
}
