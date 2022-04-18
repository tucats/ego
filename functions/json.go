package functions

import (
	"encoding/json"
	"strings"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Decode reads a string as JSON data.
func Decode(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var v interface{}

	jsonBuffer := datatypes.GetString(args[0])
	err := json.Unmarshal([]byte(jsonBuffer), &v)

	// If there is no model, assume a generic return value is okay
	if len(args) < 2 {
		// Hang on, if the result is a map, then Ego won't be able to use it,
		// so convert that to an EgoMap. Same for an array.
		if m, ok := v.(map[string]interface{}); ok {
			v = datatypes.NewMapFromMap(m)
		} else if a, ok := v.([]interface{}); ok {
			v = datatypes.NewArrayFromArray(datatypes.InterfaceType, a)
		}

		return v, errors.New(err)
	}

	// There is a model, so do some mapping if possible.
	pointer, ok := args[1].(*interface{})
	if !ok {
		return nil, errors.New(errors.ErrInvalidPointerType)
	}

	value := *pointer

	// Structure
	if target, ok := value.(*datatypes.EgoStruct); ok {
		if m, ok := v.(map[string]interface{}); ok {
			for k, v := range m {
				err = target.Set(k, v)
				if !errors.Nil(err) {
					return nil, errors.New(err)
				}
			}
		} else {
			return nil, errors.New(errors.ErrInvalidType)
		}

		return target, nil
	}

	// Map
	if target, ok := value.(*datatypes.EgoMap); ok {
		if m, ok := v.(map[string]interface{}); ok {
			for k, v := range m {
				_, err = target.Set(k, v)
				if !errors.Nil(err) {
					return nil, errors.New(err)
				}
			}
		} else {
			return nil, errors.New(errors.ErrInvalidType)
		}

		return target, nil
	}

	// Array
	if target, ok := value.(*datatypes.EgoArray); ok {
		if m, ok := v.([]interface{}); ok {
			// The target data size may be wrong, fix it
			target.SetSize(len(m))

			for k, v := range m {
				if target.ValueType().Kind() == datatypes.StructKind {
					if mm, ok := v.(map[string]interface{}); ok {
						v = datatypes.NewStructFromMap(mm)
					}
				}

				err = target.Set(k, v)
				if !errors.Nil(err) {
					return nil, errors.New(err)
				}
			}
		} else {
			return nil, errors.New(errors.ErrInvalidType)
		}

		return target, nil
	}

	if !datatypes.TypeOf(v).IsType(datatypes.TypeOf(value)) {
		err = errors.New(errors.ErrInvalidType)
		v = nil
	}

	return v, errors.New(err)
}

func Seal(i interface{}) interface{} {
	switch actualValue := i.(type) {
	case *datatypes.EgoStruct:
		actualValue.SetStatic(true)

		return actualValue

	case *datatypes.EgoArray:
		for i := 0; i <= actualValue.Len(); i++ {
			element, _ := actualValue.Get(i)
			actualValue.SetAlways(i, Seal(element))
		}

		return actualValue

	default:
		return actualValue
	}
}

// Encode writes a JSON string from arbitrary data.
func Encode(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) == 1 {
		jsonBuffer, err := json.Marshal(datatypes.Sanitize(args[0]))
		jsonString := string(jsonBuffer)

		return jsonString, errors.New(err)
	}

	var b strings.Builder

	b.WriteString("[")

	for n, v := range args {
		if n > 0 {
			b.WriteString(", ")
		}

		jsonBuffer, err := json.Marshal(datatypes.Sanitize(v))
		if !errors.Nil(err) {
			return "", errors.New(err)
		}

		b.WriteString(string(jsonBuffer))
	}

	b.WriteString("]")

	return b.String(), nil
}

// EncodeFormatted writes a  JSON string from arbitrary data.
func EncodeFormatted(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	prefix := datatypes.GetString(args[1])
	indent := datatypes.GetString(args[2])

	jsonBuffer, err := json.MarshalIndent(datatypes.Sanitize(args[0]), prefix, indent)

	return string(jsonBuffer), errors.New(err)
}
