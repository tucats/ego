package json

import (
	"encoding/json"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// marshal writes a JSON string from arbitrary data.
func marshal(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) == 1 {
		jsonBuffer, err := json.Marshal(data.Sanitize(args[0]))
		if err != nil {
			err = errors.NewError(err)
		}

		return data.List(data.NewArray(data.ByteType, 0).Append(jsonBuffer), err), err
	}

	var b strings.Builder

	b.WriteString("[")

	for n, v := range args {
		if n > 0 {
			b.WriteString(", ")
		}

		jsonBuffer, err := json.Marshal(data.Sanitize(v))
		if err != nil {
			return nil, errors.NewError(err)
		}

		b.WriteString(string(jsonBuffer))
	}

	b.WriteString("]")
	jsonBuffer := []byte(b.String())

	return data.List(data.NewArray(data.ByteType, 0).Append(jsonBuffer), nil), nil
}

// marshalIndent writes a  JSON string from arbitrary data.
func marshalIndent(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	prefix := data.String(args[1])
	indent := data.String(args[2])

	jsonBuffer, err := json.MarshalIndent(data.Sanitize(args[0]), prefix, indent)
	if err != nil {
		err = errors.NewError(err)
	}

	return data.List(data.NewArray(data.ByteType, 0).Append(jsonBuffer), err), err
}
