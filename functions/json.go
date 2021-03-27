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

	// If its a struct, make sure it has the static attribute
	v = Seal(v)

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
