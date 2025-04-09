package parser

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/tucats/ego/errors"
)

func parse(body interface{}, item string) ([]interface{}, error) {
	var (
		index   []int
		isIndex bool
		name    string
	)

	// If the item is just a "dot" it means the entire (remaining) body is the result
	if item == "." || item == "" {
		return []interface{}{body}, nil
	}

	if strings.HasPrefix(item, "..") {
		return nil, errors.ErrJSONQuery.Clone().Context(item)
	}

	item = dotQuote(strings.TrimPrefix(item, "."))

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
	if _, err := strconv.Atoi(parts[0][:1]); err == nil {
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
		result := make([]interface{}, 0, len(index))

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
