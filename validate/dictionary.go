package validate

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
)

var dictionary = map[string]interface{}{}

var dictionaryLock sync.Mutex

func Lookup(key string) interface{} {
	dictionaryLock.Lock()
	defer dictionaryLock.Unlock()

	return dictionary[key]
}

func Define(key string, object interface{}) {
	dictionaryLock.Lock()
	defer dictionaryLock.Unlock()

	dictionary[key] = object
}

func Encode(key string) ([]byte, error) {
	entry := Lookup(key)
	if entry == nil {
		return nil, errors.ErrNotFound.Clone().Context(key)
	}

	m, err := encode(entry)
	if err != nil {
		return nil, err
	}

	result, err := json.MarshalIndent(m, "", "  ")

	return result, err
}

func EncodeDictionary() ([]byte, error) {
	result := map[string]interface{}{}

	for key, entry := range dictionary {
		m, err := encode(entry)
		if err != nil {
			return nil, err
		}

		result[key] = m
	}

	b, err := json.MarshalIndent(result, "", "  ")

	return b, err
}

func encode(entry interface{}) (map[string]interface{}, error) {
	var err error

	m := map[string]interface{}{}

	switch actual := entry.(type) {
	case Item:
		m["_class"] = ItemType
		m["type"] = actual.Type
		m["name"] = actual.Name

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
			fieldMap, err := encode(field)
			if err != nil {
				return nil, err
			}

			fields[i] = fieldMap
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

		m["items"], err = encode(actual.Type)
		if err != nil {
			return nil, err
		}

	default:
		return nil, errors.ErrInvalidType.Clone().Context(actual)
	}

	return m, err
}

func LoadDictionary(filename string) error {
	b, err := os.ReadFile(filename)
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
		switch m["_class"] {
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
			array := Array{}

			item, err := decode(m["items"])
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
