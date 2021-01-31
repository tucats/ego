package symbols

import (
	"fmt"
	"strings"
)

// Error messages
const (
	Prefix = "symbol table error"

	InvalidSymbolError = "invalid symbol name"
	ReadOnlyValueError = "invalid write to read-only value"
	SymbolExistsError  = "symbol already exists"
	UnknownSymbolError = "unknown symbol"
)

// SymbolError is a symbol table manager error
type SymbolError struct {
	Text      string
	Parameter string
}

// Required interface to format an error string.
func (e *SymbolError) Error() string {
	var b strings.Builder
	b.WriteString(Prefix)
	b.WriteString(", ")
	b.WriteString(e.Text)
	if len(e.Parameter) > 0 {
		b.WriteString(": ")
		b.WriteString(e.Parameter)
	}

	return b.String()
}

// NewError creates an SymbolError object
func (*SymbolTable) NewError(text string, args ...interface{}) error {
	e := &SymbolError{Text: text}
	if len(args) > 0 {
		e.Parameter = fmt.Sprintf("%v", args[0])
	}

	return e
}
