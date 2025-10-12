package errors

import (
	nativeErrors "errors"
	"strings"

	"github.com/tucats/ego/defs"
)

// location describes a source code location. Zero-values mean
// the item is not considered present, and will be omitted from
// formatted output.
type location struct {
	name   string
	line   int
	column int
}

// Error describes the error structure shared by all of Ego.
// This includes a wrapped error (which may be from our list of
// native errors, or a return code from a Go runtime). The
// context is a string that further explains the cause/source of
// the error, and is message-specific.
type Error struct {
	err      error
	user     bool
	location *location
	context  string
	next     *Error
}

// New creates a new Error object, and fills in the native
// wrapped error. Note that if the value passed in is already
// an Error, then it is returned without re-wrapping it.
func New(err error) *Error {
	if err == nil {
		return nil
	}

	// If it's one of our errors, make a clone of it
	if e, ok := err.(*Error); ok {
		return e.Clone()
	}

	return &Error{
		err: err,
	}
}

func (e *Error) Clone() *Error {
	if e == nil {
		return nil
	}

	err := &Error{
		err:     e.err,
		context: e.context,
		next:    e.next,
	}

	if e.location != nil {
		err.location = &location{
			name:   e.location.name,
			line:   e.location.line,
			column: e.location.column,
		}
	}

	return err
}

// Chain() as a receiver function adds the err to the receiving error as
// it's dependent. If the error already has a dependent error, then the
// error is added to the end of the chain.
func (e *Error) Chain(err *Error) *Error {
	if Nil(e) || Nil(err) {
		return e
	}

	if e.next == nil {
		e.next = err
	} else {
		e.next.Chain(err)
	}

	return e
}

// Message creates a new Error using an arbitrary string.
// This is used in cases where a fmt.Errorf() was used to
// generate an error string.
func Message(m string) *Error {
	if strings.HasPrefix(m, defs.ReadonlyVariablePrefix) {
		m = m[1:]
	}

	return &Error{
		err: nativeErrors.New(m),
	}
}

func (e *Error) Code() string {
	if e == nil || e.err == nil {
		return "<nil>"
	}

	return e.err.Error()
}
