// Package tokenizer provides a language-aware tokenizer for Ego. This
// performs the functions of a lexical scanner, with the addition of doing
// parsing analysis of data types, which can set the token's class.
package tokenizer

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/tucats/ego/app-cli/ui"
)

// Tokenizer is an instance of a tokenized string.
type Tokenizer struct {
	// The source of the tokens, broken into an array of strings based on line ending characters.
	Source []string

	// The token stream for the current source text.
	Tokens []Token

	// The current position in the token stream.
	TokenP int

	// A flag that indicates if a tokenizer can be dumped to the log. This defaults to false, and
	// will only print the tokens when the compiler detects an error and the TOKENIZER log is active.
	// This can be set to true for debugging purposes.
	silent bool
}

// EndOfTokens is a reserved token that means end of the buffer was reached.
var EndOfTokens = Token{class: EndOfTokensClass}

// ToTheEnd means to advance the token stream to the end.
const ToTheEnd = 999999

// New creates a tokenizer instance and breaks the string up into an array of tokens.
// The isCode flag is used to indicate this is Ego code, which has some different
// tokenizing rules.
func New(src string, isCode bool) *Tokenizer {
	start := time.Now()
	lines := splitLines(src, isCode)
	src = strings.Join(lines, "\n")
	t := Tokenizer{
		Source: lines,
		TokenP: 0,
		silent: true,
		Tokens: make([]Token, 0),
	}

	// Do the lexical scanning to build the token stream.
	t.lexer(src, isCode)

	if !t.silent && ui.IsActive(ui.TokenLogger) {
		t.DumpTokens()
	}

	ui.Log(ui.TokenLogger, "tokens.complete", ui.A{
		"count":    len(t.Tokens),
		"duration": time.Since(start)})

	return &t
}

func (t Tokenizer) Len() int {
	return len(t.Tokens)
}

// EnableDump causes the token stream to be dumped to the console. This is only used for
// internal debugging.
func (t *Tokenizer) EnableDump() {
	t.silent = false
}

// Construct a new token given a class and spelling.
func (t *Tokenizer) NewToken(class TokenClass, spelling string) Token {
	return Token{class: ValueTokenClass, spelling: spelling}
}

// IsSymbol is a utility function to determine if a string contains is a symbol name.
func IsSymbol(s string) bool {
	for n, c := range s {
		if c == '_' || unicode.IsLetter(c) {
			continue
		}

		if n > 0 && unicode.IsDigit(c) {
			continue
		}

		return false
	}

	return true
}

// GetTokens returns a string representing the tokens within the
// given range of tokens.
func (t *Tokenizer) GetTokens(pos1, pos2 int, spacing bool) string {
	var (
		s  strings.Builder
		p1 = pos1
		p2 = pos2
	)

	if p1 < 0 {
		p1 = 0
	} else if p1 > len(t.Tokens) {
		p1 = len(t.Tokens)
	}

	if p2 < p1 {
		p2 = p1
	} else {
		if p2 > len(t.Tokens) {
			p2 = len(t.Tokens)
		}
	}

	for _, t := range t.Tokens[p1:p2] {
		s.WriteString(t.spelling)

		if spacing {
			s.WriteRune(' ')
		}
	}

	return s.String()
}

// InList is a support function that checks to see if a string matches
// any of a list of other strings.
func InList(s Token, test ...Token) bool {
	for _, t := range test {
		if s.Is(t) {
			return true
		}
	}

	return false
}

// unQuote will remove quotes and also process any escapes in the string.
// It is a wrapper around strconv.Unquote but does not report an error if
// the string is improperly formed.
func unQuote(input string) string {
	result, err := strconv.Unquote(input)
	if err != nil {
		return input
	}

	return result
}

// Close discards any storage no longer needed by the tokenizer. The
// line number and position arrays as well as the source are maintained
// to support error reporting.
func (t *Tokenizer) Close() {
	// We no longer need the token array, so free up the memory.
	t.Tokens = nil
}

// String returns a string representation of the token stream.
func (t *Tokenizer) String() string {
	return fmt.Sprintf("Tokenizer( %d tokens, %d lines)", len(t.Tokens), len(t.Source))
}
