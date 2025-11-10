package validate

import (
	"encoding/json"
	"maps"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/validator"
)

var dictionary = map[string]*validator.Item{}

var dictionaryLock sync.Mutex

func Lookup(key string) *validator.Item {
	dictionaryLock.Lock()
	defer dictionaryLock.Unlock()

	key = resolve(key)
	if key == "" {
		return nil
	}

	return dictionary[key]
}

// Exists returns true if the named type is found in the validation dictionary.
func Exists(key string) bool {
	key = resolve(key)
	if key == "" {
		return false
	}

	_, found := dictionary[key]

	return found
}
func Define(key string, object any) error {
	dictionaryLock.Lock()
	defer dictionaryLock.Unlock()

	if _, found := dictionary[key]; found {
		return errors.ErrDuplicateTypeName.Clone().Context(key)
	}

	item, err := validator.New(object)
	if err != nil {
		return err
	}

	dictionary[key] = item

	return nil
}

func DefineForeign(key string, item *validator.Item) error {
	dictionaryLock.Lock()
	defer dictionaryLock.Unlock()

	if _, found := dictionary[key]; found {
		return errors.ErrDuplicateTypeName.Clone().Context(key)
	}

	// Set the foreign key flag. If this is a pointer, set the flag on the base type.
	if item.ItemType == validator.TypePointer {
		item.BaseType.AllowForeignKey = true
	} else {
		item.AllowForeignKey = true
	}

	dictionary[key] = item

	return nil
}

func Encode(key string) ([]byte, error) {
	entry := Lookup(key)
	if entry == nil {
		return nil, errors.ErrNotFound.Clone().Context(key)
	}

	b, err := json.MarshalIndent(entry, "", "  ")

	return b, err
}

func EncodeDictionary() ([]byte, error) {
	dictionaryLock.Lock()
	defer dictionaryLock.Unlock()

	// Merge our dictionary of stored names with the validator's dictionary.
	mergedDictionary := make(map[string]*validator.Item)

	maps.Copy(mergedDictionary, dictionary)
	maps.Copy(mergedDictionary, validator.Dictionary)

	// Format it all as JSON.
	b, err := json.MarshalIndent(mergedDictionary, "", "  ")

	return b, err
}

func LoadDictionary(filename string) error {
	b, err := ui.ReadJSONFile(filename)
	if err != nil {
		return err
	}

	return Decode(b)
}

func Decode(b []byte) error {
	var m map[string]*validator.Item

	err := json.Unmarshal(b, &m)
	if err != nil {
		return err
	}

	// Traverse the map, finding items to put in the dictionary.
	for key, value := range m {
		if value == nil {
			continue
		}

		dictionary[key] = value
	}

	return nil
}

// Resolve a name, including traversing aliases.
func resolve(key string) string {
	_, found := dictionary[key]
	if !found {
		return ""
	}

	return key
}
