package strings

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// tokenize splits a string into tokens.
func tokenize(s *symbols.SymbolTable, args data.List) (any, error) {
	src := data.String(args.Get(0))
	t := tokenizer.New(src, false)

	r := data.NewArray(data.ArrayType(data.StructType), len(t.Tokens))

	for i, n := range t.Tokens {
		item := data.NewStructFromMap(
			map[string]any{
				"kind":     n.Class().String(),
				"spelling": n.Spelling(),
			},
		).SetFieldOrder([]string{"kind", "spelling"})

		if err := r.Set(i, item); err != nil {
			return nil, err
		}
	}

	return r, nil
}
