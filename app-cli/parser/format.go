package parser

import (
	"encoding/json"
	"fmt"
	"math"
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

	// If it's a float, see if it should really be formatted
	// as an integer.
	if f, ok := item.(float64); ok {
		i := math.Floor(f)
		if i == f && math.Abs(i) < float64(math.MaxInt-1) {
			item = int(i)
		}
	}

	// Format it as the base object type.
	return []string{fmt.Sprintf("%v", item)}, nil
}
