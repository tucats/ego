package sqlparse

// This file defines the lexical token vocabulary: the small, fixed set of
// "shapes" that lexer.go sorts every piece of source text into (see that
// file's comment for what tokenizing means and why it's a separate pass
// from parsing). SQL keywords are not reserved at the lexer level — a
// keyword such as SELECT or WHERE is lexed as an ordinary tokIdent, exactly
// like a table or column name would be, and the parser is the one that
// recognizes keywords, by comparing token text case-insensitively at the
// point in the grammar where a keyword is expected (see isKeyword and
// isKeywordAt in parser.go). This mirrors how both sqlite3 and PostgreSQL
// treat most keywords as contextual rather than fully reserved — you really
// can have a column named "value" or "type" in both — and it keeps the
// lexer itself small and dialect-agnostic: it doesn't need a table of every
// SQL keyword, only a table of punctuation and operators.

// tokenKind classifies a lexical token. Every token produced by the lexer
// is one of exactly these eight kinds; see the token struct below for what
// actually gets attached to a token of each kind.
type tokenKind int

const (
	tokEOF         tokenKind = iota
	tokIdent                 // bare or quoted identifier; Text is the resolved name
	tokNumber                // integer or float literal; Text is the raw spelling
	tokString                // string literal; Text is the decoded value
	tokBlob                  // X'...' hex blob literal; Text is the hex digits
	tokPlaceholder           // bind parameter; Text is the raw spelling incl. marker
	tokPunct                 // one of ( ) , .  ;
	tokOp                    // an operator, symbolic or word-based normalized form
)

func (k tokenKind) String() string {
	switch k {
	case tokEOF:
		return "EOF"
	case tokIdent:
		return "identifier"
	case tokNumber:
		return "number"
	case tokString:
		return "string"
	case tokBlob:
		return "blob"
	case tokPlaceholder:
		return "placeholder"
	case tokPunct:
		return "punctuation"
	case tokOp:
		return "operator"
	default:
		return "token"
	}
}

// token is one lexical token together with its source position and, for
// quoted identifiers, whether it was quoted (which suppresses keyword
// recognition — a quoted "select" is a column named select, not the SELECT
// keyword).
type token struct {
	kind   tokenKind
	text   string
	quoted bool
	line   int
	col    int
}

// is reports whether the token is an (unquoted) identifier whose text
// matches word case-insensitively — the mechanism the parser uses to
// recognize keywords.
func (t token) is(word string) bool {
	return t.kind == tokIdent && !t.quoted && equalFold(t.text, word)
}

// isOp reports whether the token is the operator or punctuation spelled op.
func (t token) isOp(op string) bool {
	return (t.kind == tokOp || t.kind == tokPunct) && t.text == op
}

// equalFold is a small ASCII case-insensitive comparison, sufficient for SQL
// keyword matching (identifiers containing non-ASCII letters are never
// keywords).
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]

		if ca >= 'a' && ca <= 'z' {
			ca -= 'a' - 'A'
		}

		if cb >= 'a' && cb <= 'z' {
			cb -= 'a' - 'A'
		}

		if ca != cb {
			return false
		}
	}

	return true
}
