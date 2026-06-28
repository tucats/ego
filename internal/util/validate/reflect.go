package validate

import (
	"github.com/tucats/validator"
)

// Reflect uses the "validate:" tag on a structure instance to create validation entries
// for the item and it's nested structures or arrays. The definition is added to the
// validation dictionary.
func Reflect(name string, object any) error {
	var (
		err  error
		item *validator.Item
	)

	// See if the object is a string that contains a previously stored validation definition.
	if text, ok := object.(string); ok {
		definition, found := dictionary[text]
		if found {
			item = definition
		}
	} else {
		item, err = validator.New(object)
	}

	// IF all went well (either it was a found name, or a new validator), add it to the dictionary.
	if err == nil {
		dictionaryLock.Lock()
		dictionary[name] = item
		dictionaryLock.Unlock()
	}

	return err
}
