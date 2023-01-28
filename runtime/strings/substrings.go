package strings

import (
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

// Index implements the strings.Index() function.
func Index(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	v := data.String(args[0])
	p := data.String(args[1])

	return strings.Index(v, p) + 1, nil
}

// Substring implements the substring() function.
func Substring(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	v := data.String(args[0])

	p1 := data.Int(args[1]) // Starting character position
	if p1 < 1 {
		p1 = 1
	}

	p2 := data.Int(args[2]) // Number of characters
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

	var b strings.Builder

	pos := 1

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

// Left implements the left() function.
func Left(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var b strings.Builder

	count := 0
	v := data.String(args[0])

	p := data.Int(args[1])
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

// Right implements the right() function.
func Right(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var charPos int

	var b strings.Builder

	v := data.String(args[0])

	p := data.Int(args[1])
	if p <= 0 {
		return "", nil
	}

	// What's the actual length?
	count := 0
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

// Wrapper around strings.Contains().
func Contains(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	a := data.String(args[0])
	b := data.String(args[1])

	return strings.Contains(a, b), nil
}

// Wrapper around strings.Contains().
func ContainsAny(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	a := data.String(args[0])
	b := data.String(args[1])

	return strings.ContainsAny(a, b), nil
}

func Truncate(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	name := data.String(args[0])
	maxWidth := data.Int(args[1])

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

// Length is the strings.Length() function, which counts characters/runes instead of
// bytes like len() does.
func Length(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	count := 0
	v := data.String(args[0])

	for range v {
		count++
	}

	return count, nil
}
