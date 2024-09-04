package strings

import (
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// Wrapper around strings.fields().
func fields(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	a := data.String(args.Get(0))

	fields := strings.Fields(a)

	result := data.NewArray(data.StringType, len(fields))

	for idx, f := range fields {
		_ = result.Set(idx, f)
	}

	return result, nil
}

// tokenize splits a string into tokens.
func tokenize(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	src := data.String(args.Get(0))
	t := tokenizer.New(src, false)

	r := data.NewArray(data.ArrayType(data.StructType), len(t.Tokens))

	for i, n := range t.Tokens {
		item := data.NewStructFromMap(
			map[string]interface{}{
				"kind":     n.Class().String(),
				"spelling": n.Spelling(),
			},
		)

		if err := r.Set(i, item); err != nil {
			return nil, err
		}
	}

	return r, nil
}
