package parser

import (
	"fmt"

	"github.com/tucats/ego/errors"
)

func arrayElement(body interface{}, index int, parts []string, item string) ([]string, error) {
	var result []string

	switch actual := body.(type) {
	case []interface{}:
		if index < 0 || index >= len(actual) {
			return result, errors.ErrArrayIndex.Clone().Context(index)
		}

		return parse(actual[index], parts[1])

	case []string:
		if index < 0 || index >= len(actual) {
			return result, errors.ErrArrayIndex.Clone().Context(index)
		}

		return format(actual[index])

	case []float64:
		if index < 0 || index >= len(actual) {
			return result, errors.ErrArrayIndex.Clone().Context(index)
		}

		return format(actual[index])

	case []int:
		if index < 0 || index >= len(actual) {
			return result, errors.ErrArrayIndex.Clone().Context(index)
		}

		return format(actual[index])

	case []bool:
		if index < 0 || index >= len(actual) {
			return result, errors.ErrArrayIndex.Clone().Context(index)
		}

		return format(actual[index])

	default:
		return result, errors.ErrJSONArrayType.Clone().Context(fmt.Sprintf("%T", item))
	}
}

func anyArrayElement(body interface{}, parts []string, item string) ([]string, error) {
	var result []string

	switch actual := body.(type) {
	case []interface{}:
		for _, element := range actual {
			if text, err := parse(element, parts[1]); err == nil {
				result = append(result, text...)
			}
		}

		if len(result) > 0 {
			return result, nil
		}

		return result, errors.ErrJSONArrayNotFound.Clone().Context(item)

	case []string:
		for _, element := range actual {
			if text, err := parse(element, parts[1]); err == nil {
				result = append(result, text...)
			}
		}

		if len(result) > 0 {
			return result, nil
		}

		return nil, errors.ErrJSONArrayNotFound.Clone().Context(item)

	case []float64:
		for _, element := range actual {
			if text, err := parse(element, parts[1]); err == nil {
				result = append(result, text...)
			}
		}

		if len(result) > 0 {
			return result, nil
		}

		return result, errors.ErrJSONArrayNotFound.Clone().Context(item)

	case []int:
		for _, element := range actual {
			if text, err := parse(element, parts[1]); err == nil {
				result = append(result, text...)
			}
		}

		if len(result) > 0 {
			return result, nil
		}

		return result, errors.ErrJSONArrayNotFound.Clone().Context(item)

	case []bool:
		for _, element := range actual {
			if text, err := parse(element, parts[1]); err == nil {
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
