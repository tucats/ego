package strings

import (
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

// index implements the strings.index() function.
func index(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	v := data.String(args.Get(0))
	p := data.String(args.Get(1))

	return strings.Index(v, p) + 1, nil
}

// substring implements the substring() function.
func substring(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	var (
		b   strings.Builder
		pos = 1
	)

	v := data.String(args.Get(0))

	p1 := data.Int(args.Get(1)) // Starting character position
	if p1 < 1 {
		p1 = 1
	}

	p2 := data.Int(args.Get(2)) // Number of characters
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
func leftSubstring(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	var (
		b     strings.Builder
		count int
	)

	v := data.String(args.Get(0))

	p := data.Int(args.Get(1))
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
func rightSubstring(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	var (
		b       strings.Builder
		charPos int
		count   int
	)

	v := data.String(args.Get(0))

	p := data.Int(args.Get(1))
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

// Wrapper around strings.contains().
func contains(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	a := data.String(args.Get(0))
	b := data.String(args.Get(1))

	return strings.Contains(a, b), nil
}

// Wrapper around strings.Contains().
func containsAny(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	a := data.String(args.Get(0))
	b := data.String(args.Get(1))

	return strings.ContainsAny(a, b), nil
}

func truncate(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	name := data.String(args.Get(0))
	maxWidth := data.Int(args.Get(1))

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
func length(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	count := 0
	v := data.String(args.Get(0))

	for range v {
		count++
	}

	return count, nil
}
