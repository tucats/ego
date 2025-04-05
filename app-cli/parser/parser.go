package parser

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/tucats/ego/errors"
)

func parse(body interface{}, item string) ([]string, error) {
	var (
		index   int
		isIndex bool
		name    string
	)

	// If the item is just a "dot" it means the entire (remaining) body is the result
	if item == "." {
		// If what's left is a map, then reformat as more JSON.
		if m, ok := body.(map[interface{}]interface{}); ok {
			b, _ := json.MarshalIndent(m, "", "   ")

			return []string{string(b)}, nil
		}

		// If what's left is an array, then reformat as more JSON.
		if a, ok := body.([]interface{}); ok {
			b, _ := json.MarshalIndent(a, "", "   ")

			return []string{string(b)}, nil
		}

		return []string{fmt.Sprintf("%v", body)}, nil
	}

	item = dotQuote(item)

	// Split out the item we seek plus whatever might be after it
	parts := strings.SplitN(item, ".", 2)
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

	// Determine if this is a numeric index or a name
	if i, err := strconv.Atoi(parts[0]); err == nil {
		index = i
		isIndex = true
	} else {
		name = parts[0]
	}

	// Is the name the wildcard array index?
	if name == "*" {
		return anyArrayElement(body, parts, item)
	}

	// If it's an index, the current item must be an array
	if isIndex {
		return arrayElement(body, index, parts, item)
	}

	// If it's a name, the current item must be a map of some type.
	val := reflect.ValueOf(body)

	if val.Kind() == reflect.Map {
		for _, e := range val.MapKeys() {
			if e.String() == name {
				v := val.MapIndex(e)

				return parse(v.Interface(), parts[1])
			}
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
