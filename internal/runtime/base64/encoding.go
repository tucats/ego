package base64

import (
	"encoding/base64"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// encode encodes a string as a BASE64 string using standard encoding rules.
func encode(s *symbols.SymbolTable, args data.List) (any, error) {
	text := data.String(args.Get(0))

	return base64.StdEncoding.EncodeToString([]byte(text)), nil
}

// decode decodes a BASE64-encoded string using standard encoding rules.
// Returns a data.List of (string, error) so callers can use two-value
// assignment: s, err := base64.Decode(data)
func decode(s *symbols.SymbolTable, args data.List) (any, error) {
	text := data.String(args.Get(0))

	b, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return data.NewList(nil, errors.New(err)), nil
	}

	return data.NewList(string(b), nil), nil
}
