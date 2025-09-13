package strings

import (
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// substring implements the substring() function.
func substring(symbols *symbols.SymbolTable, args data.List) (any, error) {
	var (
		b   strings.Builder
		pos = 1
	)

	v := data.String(args.Get(0))

	p1, err := data.Int(args.Get(1)) // Starting character position
	if err != nil || p1 < 0 {
		return "", errors.New(err).In("Substring")
	}

	if p1 < 1 {
		p1 = 1
	}

	p2, err := data.Int(args.Get(2)) // Number of characters
	if err != nil || p2 < 0 {
		return "", errors.New(err).In("Substring")
	}

	if p2 == 0 {
		return "", nil
	}

	// Calculate length of v in characters
	count := 0
	for range v {
		count++
	}

	// Limit the ending bounds by the actual length
	if p2+p1 > count {
		p2 = count - p1 + 1
	}

	for _, ch := range v {
		if pos >= p1+p2 {
			break
		}

		if pos >= p1 {
			b.WriteRune(ch)
		}

		pos++
	}

	return b.String(), nil
}

// leftSubstring implements the left() function.
func leftSubstring(symbols *symbols.SymbolTable, args data.List) (any, error) {
	var (
		b     strings.Builder
		count int
	)

	v := data.String(args.Get(0))

	p, err := data.Int(args.Get(1))
	if err != nil || p <= 0 {
		return "", errors.New(err).In("Left")
	}

	if p <= 0 {
		return "", nil
	}

	for _, ch := range v {
		if count < p {
			b.WriteRune(ch)

			count++
		} else {
			break
		}
	}

	return b.String(), nil
}

// rightSubstring implements the right() function.
func rightSubstring(symbols *symbols.SymbolTable, args data.List) (any, error) {
	var (
		b       strings.Builder
		charPos int
		count   int
	)

	v := data.String(args.Get(0))

	p, err := data.Int(args.Get(1))
	if err != nil || p <= 0 {
		return "", errors.New(err).In("Right")
	}

	if p <= 0 {
		return "", nil
	}

	// What's the actual length?
	for range v {
		count++
	}

	for _, ch := range v {
		if charPos >= count-p {
			b.WriteRune(ch)
		}

		charPos++
	}

	return b.String(), nil
}

func truncate(symbols *symbols.SymbolTable, args data.List) (any, error) {
	name := data.String(args.Get(0))

	maxWidth, err := data.Int(args.Get(1))
	if err != nil {
		return nil, errors.New(err).In("Truncate")
	}

	if len(name) <= maxWidth {
		return name, nil
	}

	result := name
	chars := 0
	dots := "..."
	limit := maxWidth - len(dots) // name + `...`

	// iterating over strings is based on runes, not bytes.
	for i := range name {
		if chars >= limit {
			result = name[:i] + dots

			break
		}

		chars++
	}

	return result, nil
}

// length is the strings.length() function, which counts characters/runes instead of
// bytes like len() does.
func length(symbols *symbols.SymbolTable, args data.List) (any, error) {
	count := 0
	v := data.String(args.Get(0))

	for range v {
		count++
	}

	return count, nil
}
