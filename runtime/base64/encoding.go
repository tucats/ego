package base64

import (
	"encoding/base64"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Encode encodes a string as a BASE64 string using standard encoding rules.
func Encode(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount
	}

	text := data.String(args[0])

	return base64.StdEncoding.EncodeToString([]byte(text)), nil
}

// Decode encodes a string as a BASE64 string using standard encoding rules.
func Decode(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount
	}

	text := data.String(args[0])

	b, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return nil, errors.NewError(err)
	}

	return string(b), nil
}
