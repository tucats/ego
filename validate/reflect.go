package validate

import (
	"github.com/tucats/validator"
)

// Reflect uses the "validate:" tag on a structure instance to create validation entries
// for the item and it's nested structures or arrays. The definition is added to the
// validation dictionary.
func Reflect(name string, object any) error {
	item, err := validator.New(object)
	if err == nil {
		dictionaryLock.Lock()
		dictionary[name] = item
		dictionaryLock.Unlock()
	}

	return err
}
