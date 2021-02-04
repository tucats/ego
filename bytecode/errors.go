package bytecode

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/util"
)

// Runtime error messages.
const (
	ArgumentCountError            = "incorrect function argument count"
	ArgumentTypeError             = "incorrect function argument type"
	DivisionByZeroError           = "division by zero"
	IncorrectReturnValueCount     = "incorrect number of return values"
	InvalidArgCheckError          = "invalid ArgCheck array"
	InvalidArgTypeError           = "function argument is of wrong type"
	InvalidArrayIndexError        = "invalid array index"
	InvalidBytecodeAddress        = "invalid bytecode address"
	InvalidCallFrame              = "invalid call frame on stack"
	InvalidChannel                = "neither source or destination is a channel"
	InvalidFieldError             = "invalid field name for type"
	InvalidFunctionCallError      = "invalid function call"
	InvalidIdentifierError        = "invalid identifier"
	InvalidSliceIndexError        = "invalid slice index"
	InvalidThisError              = "invalid _this_ identifier"
	InvalidTypeError              = "invalid or unsupported data type for this operation"
	InvalidVarTypeError           = "invalid type for this variable"
	NotAServiceError              = "not running as a service"
	NotATypeError                 = "not a type"
	OpcodeAlreadyDefinedError     = "opcode already defined: %d"
	ReadOnlyError                 = "invalid write to read-only item"
	StackUnderflowError           = "stack underflow"
	TryCatchMismatchError         = "try/catch stack error"
	UnimplementedInstructionError = "unimplemented bytecode instruction"
	UnknownIdentifierError        = "unknown identifier"
	UnknownMemberError            = "unknown structure member"
	UnknownPackageMemberError     = "unknown package member"
	UnknownTypeError              = "unknown structure type"
	VarArgError                   = "invalid variable-argument operation"
)

// Error contains an error generated from the execution context.
type Error struct {
	text   string
	module string
	line   int
	token  string
}

// NewError generates a new error.
func (c *Context) NewError(msg string, args ...interface{}) *Error {
	if msg == "" {
		return nil
	}

	token := ""

	if len(args) > 0 {
		token = util.GetString(args[0])
	}

	return &Error{
		text:   msg,
		module: c.Name,
		line:   c.line,
		token:  token,
	}
}

// Error produces an error string from this object.
func (e Error) Error() string {
	var b strings.Builder

	b.WriteString("execution error ")

	if len(e.module) > 0 {
		b.WriteString("in ")
		b.WriteString(e.module)
		b.WriteString(" ")
	}

	if e.line > 0 {
		b.WriteString(fmt.Sprintf(util.LineFormat, e.line))
	}

	b.WriteString(", ")
	b.WriteString(e.text)

	if len(e.token) > 0 {
		b.WriteString(": ")
		b.WriteString(e.token)
	}

	return b.String()
}
