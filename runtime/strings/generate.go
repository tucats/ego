package strings

import (
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/termgen"
)

func generate(s *symbols.SymbolTable, args data.List) (any, error) {
	count, _ := args.GetInt(0)

	divider := ""
	if args.Len() > 1 {
		divider = data.String(args.Get(1))
	}

	pascalCase := len(divider) == 0
	if args.Len() > 2 {
		pascalCase, _ = data.Bool(args.Get(2))
	}

	words := termgen.RandomList(count)

	if pascalCase {
		for n := range len(words) {
			words[n] = uppercase(words[n])
		}
	}

	return strings.Join(words, divider), nil
}

func uppercase(s string) string {
	result := ""

	for index, ch := range s {
		if index == 0 {
			result += strings.ToUpper(string(ch))
		} else {
			result += strings.ToLower(string(ch))
		}
	}

	return result
}
