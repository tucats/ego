package validate

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
)

var dictionary = map[string]interface{}{}

var dictionaryLock sync.Mutex

func Lookup(key string) interface{} {
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
func Define(key string, object interface{}) error {
	if strings.HasPrefix(key, privateTypePrefix) {
		ui.Panic("Invalid validation definition using private prefix: " + key)
	}

	dictionaryLock.Lock()
	defer dictionaryLock.Unlock()

	if _, found := dictionary[key]; found {
		return errors.ErrDuplicateTypeName.Clone().Context(key)
	}

	dictionary[key] = object

	return nil
}

func DefineAlias(alias, original string) error {
	item := Alias{
		Type: original,
	}

	dictionaryLock.Lock()
	defer dictionaryLock.Unlock()

	if _, found := dictionary[alias]; found {
		return errors.ErrDuplicateTypeName.Clone().Context(alias)
	}

	dictionary[alias] = item

	return nil
}

func Encode(key string) ([]byte, error) {
	rootMap := make(map[string]interface{})

	entry := Lookup(key)
	if entry == nil {
		return nil, errors.ErrNotFound.Clone().Context(key)
	}

	m, newTypes, err := encode(entry)
	if err != nil {
		return nil, err
	}

	rootMap[key] = m

	for i := 0; i < len(newTypes); i++ {
		newType := newTypes[i]

		entry = Lookup(newType)
		if entry != nil {
			newM, moreTypes, err := encode(newType)
			if err != nil {
				return nil, err
			}

			rootMap[newType] = newM

			newTypes = append(newTypes, moreTypes...)
		}
	}

	result, err := json.MarshalIndent(rootMap, "", "  ")

	return result, err
}

func EncodeDictionary() ([]byte, error) {
	result := map[string]interface{}{}

	keys := make([]string, 0, len(dictionary))
	for key := range dictionary {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	for _, key := range keys {
		entry := dictionary[key]

		m, _, err := encode(entry)
		if err != nil {
			return nil, err
		}

		result[key] = m
	}

	b, err := json.MarshalIndent(result, "", "  ")

	return b, err
}

func encode(entry interface{}) (map[string]interface{}, []string, error) {
	var err error

	if entry == nil {
		return nil, nil, nil
	}

	m := map[string]interface{}{}
	types := map[string]bool{}

	switch actual := entry.(type) {
	case string:
		entry := Lookup(actual)
		if entry != nil {
			return encode(entry)
		}

	case Alias:
		m["_class"] = AliasType
		m["type"] = actual.Type
		types[actual.Type] = true

	case Item:
		m["_class"] = ItemType
		m["type"] = actual.Type
		types[actual.Type] = true

		if actual.Name != "" {
			m["name"] = actual.Name
		}

		if actual.HasMax {
			m["max"] = actual.Max
		}

		if actual.HasMin {
			m["min"] = actual.Min
		}

		if len(actual.Enum) > 0 {
			m["enum"] = actual.Enum
		}

		if actual.Required {
			m["required"] = actual.Required
		}

		if actual.MatchCase {
			m["case"] = actual.MatchCase
		}

	case Object:
		m["_class"] = ObjectType

		fields := make([]map[string]interface{}, len(actual.Fields))

		for i, field := range actual.Fields {
			var addedTypes []string

			fieldMap, addedTypes, err := encode(field)
			if err != nil {
				return nil, nil, err
			}

			fields[i] = fieldMap

			for _, t := range addedTypes {
				types[t] = true
			}
		}

		m["fields"] = fields

	case Array:
		m["_class"] = ArrayType

		if actual.Min > 0 {
			m["min"] = actual.Min
		}

		if actual.Max > 0 {
			m["max"] = actual.Max
		}

		var addedTypes []string

		m["items"], addedTypes, err = encode(actual.Type)
		if err != nil {
			return nil, nil, err
		}

		for _, t := range addedTypes {
			types[t] = true
		}

	default:
		return nil, nil, errors.ErrInvalidType.Clone().Context(actual)
	}

	newTypes := make([]string, 0, len(types))

	for t := range types {
		if !strings.HasPrefix(t, privateTypePrefix) {
			newTypes = append(newTypes, t)
		}
	}

	return m, newTypes, err
}

func LoadDictionary(filename string) error {
	b, err := ui.ReadJSONFile(filename)
	if err != nil {
		return err
	}

	return Decode(b)
}

func Decode(b []byte) error {
	var m map[string]interface{}

	err := json.Unmarshal(b, &m)
	if err != nil {
		return err
	}

	// Traverse the map, finding items to put in the dictionary.
	for key, value := range m {
		if strings.HasPrefix(key, privateTypePrefix) {
			ui.Panic("Invalid validation definition using private type prefix: " + key)
		}

		if value == nil {
			continue
		}

		entry, err := decode(value)
		if err != nil {
			return err
		}

		dictionary[key] = entry
	}

	return nil
}

func decode(value interface{}) (interface{}, error) {
	switch m := value.(type) {
	case map[string]interface{}:
		class := data.String(m["_class"])

		switch class {
		case AliasType:
			item := Alias{}
			item.Type = data.String(m["type"])

			return item, nil

		case ItemType:
			item := Item{}
			item.Type = data.String(m["type"])
			item.Name = data.String(m["name"])

			if m["max"] != nil {
				item.Max, _ = data.Int(m["max"])
				item.HasMax = true
			}

			if m["min"] != nil {
				item.Min, _ = data.Int(m["min"])
				item.HasMin = true
			}

			if m["enum"] != nil {
				list := m["enum"]
				item.Enum = list.([]interface{})
			}

			if m["required"] != nil {
				item.Required, _ = data.Bool(m["required"])
			}

			if m["case"] != nil {
				item.MatchCase, _ = data.Bool(m["case"])
			}

			return item, nil

		case ObjectType:
			object := Object{}
			fields := m["fields"].([]interface{})

			for _, value := range fields {
				field, err := decode(value)
				if err != nil {
					return nil, err
				}

				item := field.(Item)
				object.Fields = append(object.Fields, item)
			}

			return object, nil

		case ArrayType:
			var (
				err  error
				item interface{}
			)

			array := Array{}
			item, err = decode(m["items"])

			if err != nil {
				return nil, err
			}

			array.Type = item.(Item)
			array.Min, _ = data.Int(m["min"])
			array.Max, _ = data.Int(m["max"])

			return array, nil
		default:
			return nil, errors.ErrInvalidType.Clone().Context(value)
		}

	default:
		return nil, errors.ErrInvalidType.Clone().Context(value)
	}
}

// Resolve a name, including traversing aliases.
func resolve(key string) string {
	item, found := dictionary[key]
	if !found {
		return ""
	}

	if alias, ok := item.(Alias); ok {
		return resolve(alias.Type)
	}

	return key
}
