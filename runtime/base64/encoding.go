package base64

import (
	"encoding/base64"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// encode encodes a string as a BASE64 string using standard encoding rules.
func encode(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	text := data.String(args[0])

	return base64.StdEncoding.EncodeToString([]byte(text)), nil
}

// decode encodes a string as a BASE64 string using standard encoding rules.
func decode(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	text := data.String(args[0])

	b, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return nil, errors.NewError(err)
	}

	return string(b), nil
}
