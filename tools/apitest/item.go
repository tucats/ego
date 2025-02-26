package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func GetOneItem(text string, item string) (string, error) {
	items, err := GetItem(text, item)
	if err == nil {
		if len(items) == 1 {
			return items[0], nil
		}

		if len(items) > 1 {
			return "", fmt.Errorf("Ambiguious expresssion (multiple values): %s", item)
		} else {
			return "", fmt.Errorf("No such item found: %s", item)
		}
	}

	return "", err
}

// For a given JSON payload string, extract a specific item from the payload. The item specification
// is a dot-notation string that can include integer indices and string map key values. The value is
// always returned as a string representation.
func GetItem(text string, item string) ([]string, error) {
	// Convert the body text to an arbitrary interface object using JSON
	var body interface{}

	if err := json.Unmarshal([]byte(text), &body); err != nil {
		return nil, err
	}

	// If the item is just a "dot" it means the entire body is the result
	if item == "." {
		return []string{fmt.Sprintf("%v", body)}, nil
	}

	return parseItem(body, item)
}

func parseItem(body interface{}, item string) ([]string, error) {
	var (
		index   int
		isIndex bool
		name    string
	)

	// If the item is just a "dot" it means the entire body is the result
	if item == "." {
		return []string{fmt.Sprintf("%v", body)}, nil
	}

	// Split out the item we seek plus whatever might be after it
	parts := strings.SplitN(item, ".", 2)
	if len(parts) == 1 {
		parts = append(parts, ".")
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

				return parseItem(v.Interface(), parts[1])
			}
		}

		return nil, fmt.Errorf("Map element not found: %s", name)
	}

	return nil, fmt.Errorf("Item is not a map, item, or array: %T", item)
}

func arrayElement(body interface{}, index int, parts []string, item string) ([]string, error) {
	var result []string

	switch actual := body.(type) {
	case []interface{}:
		if index < 0 || index >= len(actual) {
			return result, fmt.Errorf("Index out of range: %d", index)
		}

		return parseItem(actual[index], parts[1])

	case []string:
		if index < 0 || index >= len(actual) {
			return result, fmt.Errorf("Index out of range: %d", index)
		}

		return []string{actual[index]}, nil

	case []float64:
		if index < 0 || index >= len(actual) {
			return result, fmt.Errorf("Index out of range: %d", index)
		}

		return []string{fmt.Sprintf("%v", actual[index])}, nil

	case []int:
		if index < 0 || index >= len(actual) {
			return result, fmt.Errorf("Index out of range: %d", index)
		}

		return []string{fmt.Sprintf("%v", actual[index])}, nil

	case []bool:
		if index < 0 || index >= len(actual) {
			return result, fmt.Errorf("Index out of range: %d", index)
		}

		return []string{fmt.Sprintf("%v", actual[index])}, nil

	default:
		return result, fmt.Errorf("Item is not an array: %T", item)
	}
}

func anyArrayElement(body interface{}, parts []string, item string) ([]string, error) {
	var result []string

	switch actual := body.(type) {
	case []interface{}:
		for _, element := range actual {
			if text, err := parseItem(element, parts[1]); err == nil {
				result = append(result, text...)
			}
		}

		if len(result) > 0 {
			return result, nil
		}

		return result, fmt.Errorf("Array elmeent not found: %v", item)

	case []string:
		for _, element := range actual {
			if text, err := parseItem(element, parts[1]); err == nil {
				result = append(result, text...)
			}
		}

		if len(result) > 0 {
			return result, nil
		}

		return nil, fmt.Errorf("Array elmeent not found: %v", item)

	case []float64:
		for _, element := range actual {
			if text, err := parseItem(element, parts[1]); err == nil {
				result = append(result, text...)
			}
		}

		if len(result) > 0 {
			return result, nil
		}

		return result, fmt.Errorf("Array elmeent not found: %v", item)

	case []int:
		for _, element := range actual {
			if text, err := parseItem(element, parts[1]); err == nil {
				result = append(result, text...)
			}
		}

		if len(result) > 0 {
			return result, nil
		}

		return result, fmt.Errorf("Array elmeent not found: %v", item)

	case []bool:
		for _, element := range actual {
			if text, err := parseItem(element, parts[1]); err == nil {
				result = append(result, text...)
			}
		}

		if len(result) > 0 {
			return result, nil
		}

		return result, fmt.Errorf("Array elmeent not found: %v", item)

	default:
		return result, fmt.Errorf("Item is not an array: %T", item)
	}
}
