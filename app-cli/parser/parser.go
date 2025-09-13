package parser

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/tucats/ego/errors"
)

func parse(body any, item string) ([]any, error) {
	var (
		index   []int
		isIndex bool
		name    string
	)

	// If the item is just a "dot" it means the entire (remaining) body is the result
	if item == "." || item == "" {
		return []any{body}, nil
	}

	if strings.HasPrefix(item, "..") {
		return nil, errors.ErrJSONQuery.Clone().Context(item)
	}

	item = dotQuote(strings.TrimPrefix(item, "."))

	// Split out the item we seek plus whatever might be after it
	parts := splitQuery(item, 2)
	if len(parts) == 1 {
		parts = append(parts, ".")
	}

	for i, part := range parts {
		if i == 0 {
			parts[i] = dotUnquote(part, ".")
		} else {
			parts[i] = dotUnquote(part, "\\.")
		}
	}

	// Determine if this is a numeric index or a name. Strip off any braces
	// used to illustrate array index values.
	if strings.HasPrefix(parts[0], "[") && strings.HasSuffix(parts[0], "]") {
		var err error

		parts[0] = strings.TrimPrefix(strings.TrimSuffix(parts[0], "]"), "[")

		index, err = parseSequence(parts[0])
		if err != nil {
			return nil, err
		}

		isIndex = true
	}

	if startsWithDigit(parts[0]) {
		var err error

		index, err = parseSequence(parts[0])
		if err != nil {
			return nil, err
		}

		isIndex = true
	} else {
		name = parts[0]
	}

	// Is the name the wildcard array index?
	if name == "*" {
		return anyArrayElement(body, parts[1:], item)
	}

	// If it's an index, the current item must be an array
	if isIndex {
		result := make([]any, 0, len(index))

		for _, i := range index {
			items, err := arrayElement(body, i, parts[1:], item)
			if err != nil {
				return nil, err
			}

			result = append(result, items...)
		}

		return result, nil
	}

	// If it's a name, the current item must be a map of some type.
	val := reflect.ValueOf(body)

	if val.Kind() == reflect.Map {
		var (
			hasAlternate bool
			alternate    string
		)

		if punctuation := strings.Index(name, "?"); punctuation > 0 {
			alternate = name[punctuation+1:]
			name = name[:punctuation]
			hasAlternate = true
		}

		for _, e := range val.MapKeys() {
			if e.String() == name {
				v := val.MapIndex(e)

				return parse(v.Interface(), parts[1])
			}
		}

		if hasAlternate {
			return []any{alternate}, nil
		}

		return nil, errors.ErrJSONElementNotFound.Clone().Context(name)
	}

	return nil, errors.ErrJSONInvalidContent.Clone().Context(fmt.Sprintf("%T", item))
}

func dotQuote(s string) string {
	return strings.ReplaceAll(s, "\\.", "$$DOT$$")
}

func dotUnquote(s string, target string) string {
	return strings.ReplaceAll(s, "$$DOT$$", target)
}

func splitQuery(s string, count int) []string {
	// Strip off any trailing dot
	s = strings.TrimSuffix(strings.TrimSpace(s), ".")

	// Hide away escaped brackets
	s = strings.ReplaceAll(s, "\\[", "$$LEFT-BRACKET$$")
	s = strings.ReplaceAll(s, "\\]", "$$RIGHT-BRACKET$$")

	// Convert unescaped brackets into dots so [] act as array index separators
	s = strings.ReplaceAll(s, ".[", "[")
	s = strings.ReplaceAll(s, "].", "]")
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	s = strings.ReplaceAll(s, "[", ".")
	s = strings.ReplaceAll(s, "]", ".")

	// Restore escaped brackets back into their original positions
	s = strings.ReplaceAll(s, "$$LEFT-BRACKET$$", "\\[")
	s = strings.ReplaceAll(s, "$$RIGHT-BRACKET$$", "\\]")

	// Split the resulting dot-delimited parts into individual elements
	var parts []string

	if count == 0 {
		parts = strings.Split(s, ".")
	} else {
		parts = strings.SplitN(s, ".", count)
	}

	// Remove any extra whitespace from each part and we're done.
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}

	// Remove empty leading parts
	for len(parts) > 0 && len(parts[0]) == 0 {
		parts = parts[1:]
	}

	return parts
}

func startsWithDigit(s string) bool {
	if len(s) == 0 {
		return false
	}

	return '0' <= s[0] && s[0] <= '9'
}
