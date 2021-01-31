package compiler

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/util"
)

// Error contains an error generated from the compiler
type Error struct {
	text   string
	pkg    string
	line   int
	column int
	token  string
}

// Compiler errors. Currently these are the string values. They will eventually
// be converted to identifiers for localized assets.
const (
	BlockQuoteError                = "invalid block quote"
	FunctionAlreadyExistsError     = "function already defined"
	GenericError                   = "general error"
	InvalidChannelList             = "invalid use of assignment list for channel"
	InvalidConstantError           = "invalid constant expression"
	InvalidDirectiveError          = "invalid directive name"
	InvalidFunctionArgument        = "invalid function argument"
	InvalidFunctionCall            = "invalid function invocation"
	InvalidFunctionName            = "invalid function name"
	InvalidImportError             = "import not permitted inside a block or loop"
	InvalidListError               = "invalid list"
	InvalidLoopControlError        = "loop control statement outside of for-loop"
	InvalidLoopIndexError          = "invalid loop index variable"
	InvalidRangeError              = "invalid range"
	InvalidReturnTypeList          = "invalid return type list"
	InvalidReturnValueError        = "invalid return value for void function"
	InvalidSymbolError             = "invalid symbol name"
	InvalidTypeCheckError          = "invalid @type keyword"
	InvalidTypeSpecError           = "invalid type specification"
	InvalidTypeNameError           = "invalid type name"
	LoopBodyError                  = "for{} body empty"
	LoopExitError                  = "for{} has no exit"
	MissingAssignmentError         = "missing '=' or ':='"
	MissingBracketError            = "missing array bracket"
	MissingBlockError              = "missing '{'"
	MissingCaseError               = "missing 'case'"
	MissingCatchError              = "missing 'catch' clause"
	MissingColonError              = "missing ':'"
	MissingEndOfBlockError         = "missing '}'"
	MissingEqualError              = "missing '='"
	MissingForLoopInitializerError = "missing for-loop initializer"
	MissingFunctionTypeError       = "missing function return type"
	MissingLoopAssignmentError     = "missing ':='"
	MissingParenthesisError        = "missing parenthesis"
	MissingReturnValues            = "missing return values"
	MissingSemicolonError          = "missing ';'"
	MissingTermError               = "missing term"
	PackageRedefinitionError       = "cannot redefine existing package"
	TestingAssertError             = "testing @assert failure"
	TooManyReturnValues            = "too many return values"
	UnexpectedTokenError           = "unexpected token"
	UnrecognizedStatementError     = "unrecognized statement"
	WrongModeError                 = "directive invalid for mode"
)

// NewError generates a new compiler error
func (c *Compiler) NewError(msg string, args ...interface{}) *Error {
	p := c.t.TokenP
	if p < 0 {
		p = 0
	}
	if p >= len(c.t.Tokens) {
		p = len(c.t.Tokens) - 1
	}
	token := ""
	if len(args) > 0 {
		token = util.GetString(args[0])
	}
	e := &Error{
		text:  msg,
		token: token,
		pkg:   c.PackageName,
	}
	if p >= 0 {
		e.line = c.t.Line[p]
		e.column = c.t.Pos[p]
	}

	return e
}

// Error produces an error string from this object.
func (e Error) Error() string {
	var b strings.Builder
	b.WriteString("compile error ")
	if e.pkg != "" {
		b.WriteString("in package ")
		b.WriteString(e.pkg)
		b.WriteString(" ")
	}
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
