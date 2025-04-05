package parser

import (
	"encoding/json"
	"fmt"
)

func format(item interface{}) ([]string, error) {
	// If the item is a map, then reformat as more JSON.
	if m, ok := item.(map[string]interface{}); ok {
		b, _ := json.MarshalIndent(m, "", "   ")

		return []string{string(b)}, nil
	}

	// If the item is a more opaque map, then reformat as more JSON.
	if m, ok := item.(map[interface{}]interface{}); ok {
		b, _ := json.MarshalIndent(m, "", "   ")

		return []string{string(b)}, nil
	}

	// If the item is an array, then reformat as more JSON.
	if a, ok := item.([]interface{}); ok {
		b, _ := json.MarshalIndent(a, "", "   ")

		return []string{string(b)}, nil
	}

	// Format it as the base object type.
	return []string{fmt.Sprintf("%v", item)}, nil
}
