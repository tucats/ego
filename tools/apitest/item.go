package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// For a given JSON payload string, extract a specific item from the payload. The item specification
// is a dot-notation string that can include integer indices and string map key values. The value is
// always returned as a string representation.
func GetItem(text string, item string) (string, error) {
	// Convert the body text to an arbitrary interface object using JSON
	var body interface{}

	if err := json.Unmarshal([]byte(text), &body); err != nil {
		return "", err
	}

	// If the item is just a "dot" it means the entire body is the result
	if item == "." {
		return fmt.Sprintf("%v", body), nil
	}

	return parseItem(body, item)
}

func parseItem(body interface{}, item string) (string, error) {
	var (
		index   int
		isIndex bool
		name    string
	)

	// If the item is just a "dot" it means the entire body is the result
	if item == "." {
		return fmt.Sprintf("%v", body), nil
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

	// If it's an index, the current item must be an array
	if isIndex {
		switch actual := body.(type) {
		case []interface{}:
			if index < 0 || index >= len(actual) {
				return "", fmt.Errorf("Index out of range: %d", index)
			}

			return parseItem(actual[index], parts[1])

		case []string:
			if index < 0 || index >= len(actual) {
				return "", fmt.Errorf("Index out of range: %d", index)
			}

			return actual[index], nil

		case []float64:
			if index < 0 || index >= len(actual) {
				return "", fmt.Errorf("Index out of range: %d", index)
			}

			return fmt.Sprintf("%v", actual[index]), nil

		case []int:
			if index < 0 || index >= len(actual) {
				return "", fmt.Errorf("Index out of range: %d", index)
			}

			return fmt.Sprintf("%v", actual[index]), nil

		case []bool:
			if index < 0 || index >= len(actual) {
				return "", fmt.Errorf("Index out of range: %d", index)
			}

			return fmt.Sprintf("%v", actual[index]), nil

		default:
			return "", fmt.Errorf("Item is not an array: %T", item)
		}
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

		return "", fmt.Errorf("Map element not found: %s", name)
	}

	return "", fmt.Errorf("Item is not a map, item, or array: %T", item)
}
