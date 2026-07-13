package strings

import (
	"unicode/utf8"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// tokenize splits a string into tokens.
//
// tokenizer.New is called with isCode=true so that multi-character operator
// tokens like "{}" and "<-" are merged into single tokens, per the documented
// behavior of strings.Tokenize (BUG-51). That flag also enables Go-style
// automatic semicolon insertion, which is meant for compiling whole programs
// and is not part of Tokenize's documented contract, so any semicolon token
// synthesized by that pass (rather than typed by the caller) is filtered out
// of the result below.
func tokenize(s *symbols.SymbolTable, args data.List) (any, error) {
	src := data.String(args.Get(0))
	t := tokenizer.New(src, true)

	items := make([]any, 0, len(t.Tokens))

	for _, n := range t.Tokens {
		if isSyntheticSemicolon(t, n) {
			continue
		}

		item := data.NewStructFromMap(
			map[string]any{
				"kind":     n.Class().String(),
				"spelling": n.Spelling(),
			},
		).SetFieldOrder([]string{"kind", "spelling"})

		items = append(items, item)
	}

	r := data.NewArray(StringsTokenArrayType, len(items))

	for i, item := range items {
		if err := r.Set(i, item); err != nil {
			return nil, err
		}
	}

	return r, nil
}

// isSyntheticSemicolon reports whether tok is a semicolon that was inserted
// automatically by the tokenizer's Go-style auto-semicolon pass rather than
// one the caller actually typed. Auto-inserted semicolons are appended after
// the end of the original line's text (see tokenizer's lineEnding handling),
// so their column position falls beyond the length of the line as returned
// by GetLine, which has the synthetic suffix already stripped. A semicolon
// the caller typed always has a position within that original text.
func isSyntheticSemicolon(t *tokenizer.Tokenizer, tok tokenizer.Token) bool {
	if tok.IsNot(tokenizer.SemicolonToken) {
		return false
	}

	line, pos := tok.Location()

	return pos > utf8.RuneCountInString(t.GetLine(line))
}
