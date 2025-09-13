package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/ego/errors"
)

func arrayElement(body any, index int, parts []string, item string) ([]any, error) {
	var (
		result   []any
		subQuery string
	)

	if len(parts) > 0 {
		subQuery = strings.Join(parts, ".")
	}

	switch actual := body.(type) {
	case []any:
		if index < 0 || index >= len(actual) {
			return result, errors.ErrArrayIndex.Clone().Context(index)
		}

		return parse(actual[index], subQuery)

	case []string:
		if index < 0 || index >= len(actual) {
			return result, errors.ErrArrayIndex.Clone().Context(index)
		}

		return []any{actual[index]}, nil

	case []float64:
		if index < 0 || index >= len(actual) {
			return result, errors.ErrArrayIndex.Clone().Context(index)
		}

		return []any{actual[index]}, nil

	case []int:
		if index < 0 || index >= len(actual) {
			return result, errors.ErrArrayIndex.Clone().Context(index)
		}

		return []any{actual[index]}, nil

	case []bool:
		if index < 0 || index >= len(actual) {
			return result, errors.ErrArrayIndex.Clone().Context(index)
		}

		return []any{actual[index]}, nil

	default:
		return result, errors.ErrJSONArrayType.Clone().Context(fmt.Sprintf("%T", item))
	}
}

func anyArrayElement(body any, parts []string, item string) ([]any, error) {
	var (
		query    string
		result   []any
		subQuery string
	)

	if len(parts) > 0 {
		query = parts[0]
		// See if the query string ends with a "." followed by an integer value.
		queryParts := splitQuery(query, 0)
		if len(queryParts) > 1 {
			lastItem := queryParts[len(queryParts)-1]
			if _, err := strconv.Atoi(lastItem); err == nil {
				query = strings.Join(queryParts[:len(queryParts)-1], ".")
				parts = append([]string{query, lastItem}, parts[1:]...)
			}
		}
	}

	if len(parts) > 1 {
		subQuery = strings.Join(parts[1:], ".")
	}

	switch actual := body.(type) {
	case []any:
		for _, element := range actual {
			if text, err := parse(element, query); err == nil {
				result = append(result, text...)
			}
		}

		if len(result) > 0 {
			if subQuery != "" {
				return parse(result, subQuery)
			} else {
				return result, nil
			}
		}

		return result, errors.ErrJSONArrayNotFound.Clone().Context(item)

	case []string:
		for _, element := range actual {
			if text, err := parse(element, query); err == nil {
				result = append(result, text...)
			}
		}

		if len(result) > 0 {
			return result, nil
		}

		return nil, errors.ErrJSONArrayNotFound.Clone().Context(item)

	case []float64:
		for _, element := range actual {
			if text, err := parse(element, query); err == nil {
				result = append(result, text...)
			}
		}

		if len(result) > 0 {
			return result, nil
		}

		return result, errors.ErrJSONArrayNotFound.Clone().Context(item)

	case []int:
		for _, element := range actual {
			if text, err := parse(element, query); err == nil {
				result = append(result, text...)
			}
		}

		if len(result) > 0 {
			return result, nil
		}

		return result, errors.ErrJSONArrayNotFound.Clone().Context(item)

	case []bool:
		for _, element := range actual {
			if text, err := parse(element, query); err == nil {
				result = append(result, text...)
			}
		}

		if len(result) > 0 {
			return result, nil
		}

		return result, errors.ErrJSONArrayNotFound.Clone().Context(item)

	default:
		return result, errors.ErrJSONArrayType.Clone().Context(fmt.Sprintf("%T", item))
	}
}
