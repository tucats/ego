package strings

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// format implements the strings.format() function.
func format(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) == 0 {
		return "", nil
	}

	if len(args) == 1 {
		return data.String(args[0]), nil
	}

	return fmt.Sprintf(data.String(args[0]), args[1:]...), nil
}

// chars implements the strings.chars() function. This accepts a string
// value and converts it to an array of characters.
func chars(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	count := 0

	// Count the number of characters in the string. (We can't use len() here
	// which onl returns number of bytes)
	v := data.String(args[0])
	for i := range v {
		count = i + 1
	}

	r := data.NewArray(data.StringType, count)

	for i, ch := range v {
		err := r.Set(i, string(ch))
		if err != nil {
			return nil, err
		}
	}

	return r, nil
}

// extractInts implements the strings.ints() function. This accepts a string
// value and converts it to an array of integer rune values.
func extractInts(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	count := 0

	// Count the number of characters in the string. (We can't use len() here
	// which onl returns number of bytes)
	v := data.String(args[0])
	for i := range v {
		count = i + 1
	}

	r := data.NewArray(data.IntType, count)

	for i, ch := range v {
		err := r.Set(i, int(ch))
		if err != nil {
			return nil, err
		}
	}

	return r, nil
}

// toString implements the strings.toString() function, which accepts an array
// of items and converts it to a single long string of each item. Normally , this is
// an array of characters.
func toString(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var b strings.Builder

	for _, v := range args {
		switch a := v.(type) {
		case string:
			b.WriteString(a)

		case byte:
			b.WriteRune(rune(a))

		case int32:
			b.WriteRune(a)

		case int:
			b.WriteRune(rune(a))

		default:
			return nil, errors.ErrArgumentCount.In("String()")
		}
	}

	return b.String(), nil
}
