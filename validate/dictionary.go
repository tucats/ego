package validate

import (
	"encoding/json"
	"sync"

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

func encode(entry interface{}) (map[string]interface{}, error) {
	var err error

	m := map[string]interface{}{}

	switch actual := entry.(type) {
	case Item:
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

		m["required"] = actual.Required
		m["case"] = actual.MatchCase

	case Object:
		m["type"] = ObjectType

		fields := make([]map[string]interface{}, len(actual.Fields))
		for i, field := range actual.Fields {
			fieldMap, err := encode(field)
			if err != nil {
				return nil, err
			}

			fields[i] = fieldMap
		}

	case Array:
		m["type"] = ArrayType

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
