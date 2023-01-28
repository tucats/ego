package strings

import (
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// Wrapper around strings.Fields().
func Fields(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	a := data.String(args[0])

	fields := strings.Fields(a)

	result := data.NewArray(data.StringType, len(fields))

	for idx, f := range fields {
		_ = result.Set(idx, f)
	}

	return result, nil
}

// Wrapper around strings.Count().
func Count(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	a := data.String(args[0])
	b := data.String(args[1])

	return strings.Count(a, b), nil
}

// Tokenize splits a string into tokens.
func Tokenize(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	src := data.String(args[0])
	t := tokenizer.New(src, false)

	r := data.NewArray(data.StringType, len(t.Tokens))

	var err error

	for i, n := range t.Tokens {
		err = r.Set(i, n)
		if err != nil {
			return nil, err
		}
	}

	return r, err
}
