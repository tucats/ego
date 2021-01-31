package expressions

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/util"
)

// Error message strings
const (
	BlockQuoteError         = "invalid block quote terminator"
	GeneralExpressionError  = "general expression error"
	InvalidListError        = "invalid list"
	InvalidRangeError       = "invalid array range"
	InvalidSymbolError      = "invalid symbol name"
	MismatchedQuoteError    = "mismatched quote error"
	MissingBracketError     = "missing or invalid '[]'"
	MissingColonError       = "missing ':'"
	MissingParenthesisError = "missing parenthesis"
	MissingTermError        = "missing term"
	UnexpectedTokenError    = "unexpected token"
)

// Error contains an error generated from the compiler
type Error struct {
	text   string
	line   int
	column int
	token  string
}

// Error produces an error string from this object.
func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString("compile error ")
	if e.line > 0 {
		b.WriteString(fmt.Sprintf(util.LineColumnFormat, e.line, e.column))
	}
	b.WriteString(", ")
	b.WriteString(e.text)
	if len(e.token) > 0 {
		b.WriteString(": ")
		b.WriteString(e.token)
	}

	return b.String()
}
