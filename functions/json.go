package functions

import (
	"encoding/json"
	"strings"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// Decode reads a string as JSON data.
func Decode(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var v interface{}

	jsonBuffer := util.GetString(args[0])
	err := json.Unmarshal([]byte(jsonBuffer), &v)

	// If there is no model, assume a generic return value is okay
	if len(args) < 2 {
		return v, errors.New(err)
	}

	// There is a model, so do some mapping if possible.
	pointer, ok := args[1].(*interface{})
	if !ok {
		return nil, errors.New(errors.InvalidPointerTypeError)
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
			return nil, errors.New(errors.InvalidTypeError)
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
			return nil, errors.New(errors.InvalidTypeError)
		}

		return target, nil
	}

	// Array
	if target, ok := value.(*datatypes.EgoArray); ok {
		if m, ok := v.([]interface{}); ok {
			// The target data size may be wrong, fix it
			target.SetSize(len(m))

			for k, v := range m {
				err = target.Set(k, v)
				if !errors.Nil(err) {
					return nil, errors.New(err)
				}
			}
		} else {
			return nil, errors.New(errors.InvalidTypeError)
		}

		return target, nil
	}

	if !datatypes.TypeOf(v).IsType(datatypes.TypeOf(value)) {
		err = errors.New(errors.InvalidTypeError)
		v = nil
	}

	return v, errors.New(err)
}

func Seal(i interface{}) interface{} {
	switch actualValue := i.(type) {
	case *datatypes.EgoStruct:
		actualValue.SetStatic(true)

		return actualValue

	case map[string]interface{}:
		for k, v := range actualValue {
			actualValue[k] = Seal(v)
		}

		datatypes.SetMetadata(actualValue, datatypes.StaticMDKey, true)

		return actualValue

	case []interface{}:
		for k, v := range actualValue {
			actualValue[k] = Seal(v)
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
	if len(args) == 1 {
		jsonBuffer, err := json.MarshalIndent(datatypes.Sanitize(args[0]), "", "  ")

		return string(jsonBuffer), errors.New(err)
	}

	var b strings.Builder

	b.WriteString("[")

	for n, v := range args {
		if n > 0 {
			b.WriteString(", ")
		}

		jsonBuffer, err := json.MarshalIndent(datatypes.Sanitize(v), "", "  ")
		if !errors.Nil(err) {
			return "", errors.New(err)
		}

		b.WriteString(string(jsonBuffer))
	}

	b.WriteString("]")

	return b.String(), nil
}
