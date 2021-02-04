package functions

import (
	"encoding/json"
	"strings"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// Decode reads a string as JSON data.
func Decode(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var v interface{}

	jsonBuffer := util.GetString(args[0])
	err := json.Unmarshal([]byte(jsonBuffer), &v)

	// If its a struct, make sure it has the static attribute
	v = Seal(v)

	return v, err
}

func Seal(i interface{}) interface{} {
	switch actualValue := i.(type) {
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

// Encode writes a  JSON string from arbitrary data.
func Encode(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) == 1 {
		jsonBuffer, err := json.Marshal(args[0])

		return string(jsonBuffer), err
	}

	var b strings.Builder

	b.WriteString("[")

	for n, v := range args {
		if n > 0 {
			b.WriteString(", ")
		}

		jsonBuffer, err := json.Marshal(v)
		if err != nil {
			return "", err
		}

		b.WriteString(string(jsonBuffer))
	}

	b.WriteString("]")

	return b.String(), nil
}

// EncodeFormatted writes a  JSON string from arbitrary data.
func EncodeFormatted(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) == 1 {
		jsonBuffer, err := json.MarshalIndent(args[0], "", "  ")

		return string(jsonBuffer), err
	}

	var b strings.Builder

	b.WriteString("[")

	for n, v := range args {
		if n > 0 {
			b.WriteString(", ")
		}

		jsonBuffer, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return "", err
		}

		b.WriteString(string(jsonBuffer))
	}

	b.WriteString("]")

	return b.String(), nil
}
