package sqlparse

import (
	"strings"
	"unicode"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/sqlparse/ast"
)

// lexer turns SQL source text into a flat token list. This file is the
// "lexer" or "tokenizer" (the two terms mean the same thing here) — it does
// not know anything about SQL grammar (it has no idea SELECT must be
// followed by a column list, or that FROM introduces a table). Its only job
// is the much smaller one of chopping the raw character stream into the
// smallest meaningful pieces — identifiers, numbers, string literals,
// punctuation, operators — and throwing away the parts that carry no
// meaning (whitespace, comments). That gives the parser (parser.go and the
// other files in this package) a much simpler job: it works over a sequence
// of typed, positioned tokens instead of raw runes, so it never has to
// think about "is this a space" or "is this the start of a comment" — only
// "is this token a SELECT keyword or an identifier or a comma".
//
// This lexer scans the entire source in one pass up front and returns the
// whole token slice (see tokenize below), rather than being an on-demand
// scanner that the parser pulls one token at a time from mid-parse (like
// tokenizer.Tokenizer in internal/language/tokenizer, which this design is
// modeled on). Concretely, that means the parser can look as many tokens
// ahead as it wants just by indexing further into the slice (see
// parser.peek in parser.go) instead of needing a rewindable stream. A whole
// SQL statement's tokens comfortably fit in memory, so there's no downside
// to scanning it all up front.
type lexer struct {
	// src holds the source text as a slice of runes rather than the raw
	// string (or a []byte). A Go string is just bytes, and SQL source may
	// contain multi-byte UTF-8 characters (accented letters in a string
	// literal, for instance); indexing a string by byte position risks
	// landing in the middle of one such character. Converting once up front
	// with []rune(source) decodes the whole string into Unicode code points,
	// so pos below always refers to a whole character, and every method on
	// lexer can safely index src directly.
	src []rune

	// pos is the index into src of the next character to be read — the
	// lexer's "cursor". peek/peekAt look at src[pos+offset] without moving
	// it; advance reads src[pos] and then moves it forward by one. This
	// same peek-then-advance shape reappears, one level up, in the parser's
	// own cursor over tokens (see parser.peek/parser.next in parser.go) —
	// it's the standard way to implement bounded lookahead over a sequence.
	pos int

	// line and col track the current source position in human terms
	// (1-based line and column) purely for error reporting; they're kept in
	// sync with pos by advance, which bumps col by one per character except
	// on '\n', where it resets col to 1 and bumps line instead.
	line int
	col  int

	dialect ast.Dialect
}

func newLexer(source string, dialect ast.Dialect) *lexer {
	return &lexer{
		src:     []rune(source),
		pos:     0,
		line:    1,
		col:     1,
		dialect: dialect,
	}
}

// tokenize scans the entire source and returns its token list, terminated by
// a single tokEOF token. It returns an error at the first lexical error
// (unterminated string/comment/identifier, or an unrecognized character).
func (l *lexer) tokenize() ([]token, error) {
	var toks []token

	for {
		if err := l.skipTrivia(); err != nil {
			return nil, err
		}

		if l.atEnd() {
			toks = append(toks, token{kind: tokEOF, line: l.line, col: l.col})

			return toks, nil
		}

		tok, err := l.scanToken()
		if err != nil {
			return nil, err
		}

		toks = append(toks, tok)
	}
}

func (l *lexer) atEnd() bool {
	return l.pos >= len(l.src)
}

func (l *lexer) peek() rune {
	return l.peekAt(0)
}

func (l *lexer) peekAt(offset int) rune {
	p := l.pos + offset
	if p < 0 || p >= len(l.src) {
		return 0
	}

	return l.src[p]
}

func (l *lexer) advance() rune {
	r := l.src[l.pos]
	l.pos++

	if r == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}

	return r
}

// skipTrivia consumes whitespace, "--" line comments, and "/* */" block
// comments, stopping at the first character of the next real token.
func (l *lexer) skipTrivia() error {
	for !l.atEnd() {
		r := l.peek()

		switch {
		case r == ' ' || r == '\t' || r == '\r' || r == '\n':
			l.advance()
		case r == '-' && l.peekAt(1) == '-':
			l.advance()
			l.advance()

			for !l.atEnd() && l.peek() != '\n' {
				l.advance()
			}
		case r == '/' && l.peekAt(1) == '*':
			startLine, startCol := l.line, l.col

			l.advance()
			l.advance()

			closed := false

			for !l.atEnd() {
				if l.peek() == '*' && l.peekAt(1) == '/' {
					l.advance()
					l.advance()

					closed = true

					break
				}

				l.advance()
			}

			if !closed {
				return errors.New(errors.ErrSQLUnterminatedComment).At(startLine, startCol)
			}
		default:
			return nil
		}
	}

	return nil
}

// scanToken looks at (without consuming) the next character to decide what
// *kind* of token starts here, then delegates to the matching scanX helper
// to actually consume it and build the token. line and col are captured
// before that dispatch, so they always mark the token's first character —
// every scanX helper below takes them as parameters and stamps them onto
// the token it returns, rather than each one recomputing "where did I
// start". This dispatch-by-lookahead-character approach is possible because
// SQL tokens are (almost) uniquely identified by their first character: a
// digit always starts a number, a quote always starts a string, and so on.
// The two-character lookahead cases below (e.g. '$' only starts a
// placeholder if a digit follows) exist for the handful of places where one
// character isn't enough to decide.
func (l *lexer) scanToken() (token, error) {
	line, col := l.line, l.col
	r := l.peek()

	switch {
	case isIdentStart(r):
		return l.scanIdentOrPrefixedLiteral(line, col)
	case r == '"' || r == '`' || r == '[':
		return l.scanQuotedIdent(line, col)
	case r == '\'':
		return l.scanString(line, col, false)
	case unicode.IsDigit(r) || (r == '.' && unicode.IsDigit(l.peekAt(1))):
		return l.scanNumber(line, col)
	case r == '?':
		return l.scanPlaceholder(line, col)
	case r == ':' && isIdentStart(l.peekAt(1)):
		return l.scanNamedPlaceholder(line, col)
	case r == '@' && isIdentStart(l.peekAt(1)):
		return l.scanNamedPlaceholder(line, col)
	case r == '$' && unicode.IsDigit(l.peekAt(1)):
		return l.scanNumberedDollarPlaceholder(line, col)
	case r == '(' || r == ')' || r == ',' || r == ';' || r == '.':
		l.advance()

		return token{kind: tokPunct, text: string(r), line: line, col: col}, nil
	default:
		return l.scanOperator(line, col)
	}
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentCont(r rune) bool {
	return r == '_' || r == '$' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// scanIdentOrPrefixedLiteral scans a bare identifier, then checks for the
// two single-letter prefixes that change meaning when immediately (no
// intervening whitespace) followed by a quote: X'...' / x'...' (a hex blob
// literal) and E'...' (a PostgreSQL backslash-escape string literal, also
// accepted from sqlite3 source for leniency).
func (l *lexer) scanIdentOrPrefixedLiteral(line, col int) (token, error) {
	start := l.pos

	for !l.atEnd() && isIdentCont(l.peek()) {
		l.advance()
	}

	text := string(l.src[start:l.pos])

	if l.peek() == '\'' {
		if len(text) == 1 && (text[0] == 'x' || text[0] == 'X') {
			return l.scanBlob(line, col)
		}

		if len(text) == 1 && (text[0] == 'e' || text[0] == 'E') {
			return l.scanString(line, col, true)
		}
	}

	return token{kind: tokIdent, text: text, line: line, col: col}, nil
}

func (l *lexer) scanQuotedIdent(line, col int) (token, error) {
	open := l.advance()

	closeChar := open
	if open == '[' {
		closeChar = ']'
	}

	var b strings.Builder

	for {
		if l.atEnd() {
			return token{}, errors.New(errors.ErrSQLUnterminatedIdentifier).At(line, col)
		}

		r := l.advance()

		if r == closeChar {
			// Doubled delimiter (e.g. "" inside "...") is an escaped literal
			// delimiter character, not the closing quote — but bracket
			// identifiers have no escape convention.
			if closeChar != ']' && l.peek() == closeChar {
				l.advance()
				b.WriteRune(closeChar)

				continue
			}

			break
		}

		b.WriteRune(r)
	}

	return token{kind: tokIdent, text: b.String(), quoted: true, line: line, col: col}, nil
}

// scanString scans a single-quoted string literal. When isEscape is true the
// opening "E"/"e" prefix has already been consumed as an identifier and this
// scans a PostgreSQL-style backslash-escape string; otherwise it scans an
// ordinary SQL string, where the only escape is a doubled quote.
func (l *lexer) scanString(line, col int, isEscape bool) (token, error) {
	l.advance() // opening quote

	var b strings.Builder

	for {
		if l.atEnd() {
			return token{}, errors.New(errors.ErrSQLUnterminatedString).At(line, col)
		}

		r := l.advance()

		if r == '\'' {
			if l.peek() == '\'' {
				l.advance()
				b.WriteByte('\'')

				continue
			}

			break
		}

		if isEscape && r == '\\' {
			if l.atEnd() {
				return token{}, errors.New(errors.ErrSQLUnterminatedString).At(line, col)
			}

			b.WriteRune(l.decodeEscape())

			continue
		}

		b.WriteRune(r)
	}

	return token{kind: tokString, text: b.String(), line: line, col: col}, nil
}

// decodeEscape decodes one backslash escape sequence for an E'...' string.
// The caller has already consumed the backslash; this consumes and decodes
// the character(s) that follow it.
func (l *lexer) decodeEscape() rune {
	r := l.advance()

	switch r {
	case 'n':
		return '\n'
	case 't':
		return '\t'
	case 'r':
		return '\r'
	case 'b':
		return '\b'
	case 'f':
		return '\f'
	case '\\':
		return '\\'
	case '\'':
		return '\''
	default:
		return r
	}
}

func (l *lexer) scanBlob(line, col int) (token, error) {
	l.advance() // opening quote

	var b strings.Builder

	for {
		if l.atEnd() {
			return token{}, errors.New(errors.ErrSQLUnterminatedString).At(line, col)
		}

		r := l.advance()
		if r == '\'' {
			break
		}

		b.WriteRune(r)
	}

	return token{kind: tokBlob, text: b.String(), line: line, col: col}, nil
}

func (l *lexer) scanNumber(line, col int) (token, error) {
	start := l.pos

	if l.peek() == '0' && (l.peekAt(1) == 'x' || l.peekAt(1) == 'X') {
		l.advance()
		l.advance()

		for !l.atEnd() && isHexDigit(l.peek()) {
			l.advance()
		}

		return token{kind: tokNumber, text: string(l.src[start:l.pos]), line: line, col: col}, nil
	}

	for !l.atEnd() && unicode.IsDigit(l.peek()) {
		l.advance()
	}

	// A "." belongs to this number if it's followed by a digit (123.45), or
	// if it's a trailing decimal point not followed by another "." or the
	// start of an identifier (100. is a valid float; 100.col is not this
	// number's business — some other production will report that error).
	next := l.peekAt(1)
	if l.peek() == '.' && (unicode.IsDigit(next) || (!isIdentStart(next) && next != '.')) {
		l.advance()

		for !l.atEnd() && unicode.IsDigit(l.peek()) {
			l.advance()
		}
	}

	if l.peek() == 'e' || l.peek() == 'E' {
		la := l.peekAt(1)
		if unicode.IsDigit(la) || ((la == '+' || la == '-') && unicode.IsDigit(l.peekAt(2))) {
			l.advance()

			if l.peek() == '+' || l.peek() == '-' {
				l.advance()
			}

			for !l.atEnd() && unicode.IsDigit(l.peek()) {
				l.advance()
			}
		}
	}

	text := string(l.src[start:l.pos])
	if text == "" || text == "." {
		return token{}, errors.New(errors.ErrSQLInvalidNumber).Context(text).At(line, col)
	}

	return token{kind: tokNumber, text: text, line: line, col: col}, nil
}

func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func (l *lexer) scanPlaceholder(line, col int) (token, error) {
	start := l.pos

	l.advance() // '?'

	for !l.atEnd() && unicode.IsDigit(l.peek()) {
		l.advance()
	}

	return token{kind: tokPlaceholder, text: string(l.src[start:l.pos]), line: line, col: col}, nil
}

func (l *lexer) scanNumberedDollarPlaceholder(line, col int) (token, error) {
	start := l.pos

	l.advance() // '$'

	for !l.atEnd() && unicode.IsDigit(l.peek()) {
		l.advance()
	}

	return token{kind: tokPlaceholder, text: string(l.src[start:l.pos]), line: line, col: col}, nil
}

func (l *lexer) scanNamedPlaceholder(line, col int) (token, error) {
	start := l.pos

	l.advance() // ':' or '@'

	for !l.atEnd() && isIdentCont(l.peek()) {
		l.advance()
	}

	return token{kind: tokPlaceholder, text: string(l.src[start:l.pos]), line: line, col: col}, nil
}

// multiCharOps lists symbolic operators longer than one character, ordered
// longest-first so the scanner always matches the longest valid spelling —
// this is the standard lexer rule called "maximal munch" (or "longest
// match"): when more than one token spelling could start at the current
// position, always take the longest one. It's why this list checks "->>"
// before "->": if "->" were checked first, the input "->>"  would be
// mis-lexed as a "->" token immediately followed by a lone ">" token,
// instead of the single "->>" operator it's meant to be. The order of the
// two-character entries relative to each other doesn't matter, since none
// of them is a prefix of another.
var multiCharOps = []string{
	"->>", "::", "->", "<<", ">>", "<=", ">=", "<>", "!=", "==", "||",
}

func (l *lexer) scanOperator(line, col int) (token, error) {
	for _, op := range multiCharOps {
		if l.hasPrefix(op) {
			for range op {
				l.advance()
			}

			return token{kind: tokOp, text: op, line: line, col: col}, nil
		}
	}

	switch r := l.peek(); r {
	case '=', '<', '>', '+', '-', '*', '/', '%', '~', '&', '|':
		l.advance()

		return token{kind: tokOp, text: string(r), line: line, col: col}, nil
	default:
		l.advance()

		return token{}, errors.New(errors.ErrSQLSyntax).Context("unexpected character '"+string(r)+"'").At(line, col)
	}
}

func (l *lexer) hasPrefix(s string) bool {
	for i, r := range []rune(s) {
		if l.peekAt(i) != r {
			return false
		}
	}

	return true
}
