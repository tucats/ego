package parser

import (
	"encoding/json"

	"github.com/tucats/ego/errors"
)

func GetItem(text string, item string) (string, error) {
	items, err := GetItems(text, item)
	if err == nil {
		if len(items) == 1 {
			return items[0], nil
		}

		if len(items) > 1 {
			return "", errors.ErrJSONAmbiguous.Clone().Context(item)
		} else {
			return "", errors.ErrJSONNotFound.Clone().Context(item)
		}
	}

	return "", err
}

// For a given JSON payload string, extract a specific item from the payload. The item specification
// is a dot-notation string that can include integer indices and string map key values. The value is
// always returned as a string representation.
func GetItems(text string, item string) ([]string, error) {
	// Convert the body text to an arbitrary interface object using JSON
	var body interface{}

	if err := json.Unmarshal([]byte(text), &body); err != nil {
		return nil, err
	}

	items, err := parse(body, item)
	if err != nil {
		return nil, err
	}

	var result []string

	for _, item := range items {
		text, err := format(item)
		if err != nil {
			return nil, err
		}

		result = append(result, text...)
	}

	return result, nil
}
