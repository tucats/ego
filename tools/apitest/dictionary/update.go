package dictionary

import (
	"fmt"

	"github.com/tucats/apitest/logging"
	"github.com/tucats/apitest/parser"
)

// Update will updated (or add) items in the dictionary from the text
// of the response body, which is assumed to be a valid JSON object.
// For each item in the map, the key is used as the name of the item
// to add or update the item in the dictionary. The value of the key
// is a dot-notation string that specifies the item to extract from
// the JSON response body object.
func Update(text string, items map[string]string) error {
	for key, value := range items {
		item, err := parser.GetOneItem(text, value)
		if err != nil {
			return err
		}

		Dictionary[key] = item

		if logging.Verbose {
			if key == "API_TOKEN" {
				item = "***REDACTED***"
			}

			fmt.Printf("  Updating   {{%s}} = %s\n", key, item)
		}
	}

	return nil
}
