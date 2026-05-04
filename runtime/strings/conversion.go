package strings

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// format implements the strings.format() function.
func format(s *symbols.SymbolTable, args data.List) (any, error) {
	if args.Len() == 0 {
		return "", nil
	}

	return fmt.Sprintf(data.String(args.Get(0)), args.Elements()[1:]...), nil
}

// chars implements the strings.Chars() function. This accepts a string
// value and converts it to an array of characters.
func chars(s *symbols.SymbolTable, args data.List) (any, error) {
	v := data.String(args.Get(0))

	// Count runes, not bytes.
	count := 0
	for range v {
		count++
	}

	r := data.NewArray(data.StringType, count)

	// Use a separate rune index so multi-byte Unicode characters are placed
	// at consecutive positions rather than at their byte offsets.
	idx := 0
	for _, ch := range v {
		if err := r.Set(idx, string(ch)); err != nil {
			return nil, err
		}

		idx++
	}

	return r, nil
}

// extractInts implements the strings.Ints() function. This accepts a string
// value and converts it to an array of integer rune values.
func extractInts(s *symbols.SymbolTable, args data.List) (any, error) {
	v := data.String(args.Get(0))

	// Count runes, not bytes.
	count := 0
	for range v {
		count++
	}

	r := data.NewArray(data.IntType, count)

	// Use a separate rune index so multi-byte Unicode characters are placed
	// at consecutive positions rather than at their byte offsets.
	idx := 0
	for _, ch := range v {
		if err := r.Set(idx, int(ch)); err != nil {
			return nil, err
		}

		idx++
	}

	return r, nil
}

// toString implements the strings.toString() function, which accepts an array
// of items and converts it to a single long string of each item. Normally , this is
// an array of characters.
func toString(s *symbols.SymbolTable, args data.List) (any, error) {
	var b strings.Builder

	for _, v := range args.Elements() {
		switch a := v.(type) {
		case string:
			b.WriteString(a)

		case byte:
			b.WriteRune(rune(a))

		case int8:
			b.WriteRune(rune(a))

		case int16:
			b.WriteRune(rune(a))

		case uint16:
			b.WriteRune(rune(a))

		case int32:
			b.WriteRune(a)

		case int:
			b.WriteRune(rune(a))

		case uint:
			b.WriteRune(rune(a))

		case uint32:
			b.WriteRune(rune(a))

		case uint64:
			b.WriteRune(rune(a))

		default:
			return nil, errors.ErrArgumentCount.In("String")
		}
	}

	return b.String(), nil
}
