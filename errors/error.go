package errors

import (
	"errors"
	"fmt"
	"strings"
)

// location describes a source code location. Zero-values mean
// the item is not considered present, and will be omitted from
// formatted output.
type location struct {
	name   string
	line   int
	column int
}

// EgoErrorMsg describes the error structure shared by all of Ego.
// This includes a wrapped error (which may be from our list of
// native errors, or a return code from a Go runtime). The
// context is a string that further explains the cause/source of
// the error, and is message-specific.
type EgoErrorMsg struct {
	err      error
	location *location
	context  string
}

// EgoError creates a new EgoError object, and fils in the native
// wrapped error. Note that if the value passed in is already
// an EgoError, then it is returned without re-wrapping it.
func EgoError(err error) *EgoErrorMsg {
	if err == nil {
		return nil
	}

	if e, ok := err.(*EgoErrorMsg); ok {
		return e
	}

	return &EgoErrorMsg{
		err: err,
	}
}

// In specifies the location name. This can be the name of
// a source code module, or a function name.
func (e *EgoErrorMsg) In(name string) *EgoErrorMsg {
	if e == nil {
		return nil
	}

	if e.location == nil {
		e.location = &location{}
	}

	e.location.name = name

	return e
}

// At specifies a line number and column position related to
// the error. The line number is always present, the column
// is typically only set during compilation; if it is zero then
// it is not displayed.
func (e *EgoErrorMsg) At(line int, column int) *EgoErrorMsg {
	if e == nil {
		return nil
	}

	if e.location == nil {
		e.location = &location{}
	}

	e.location.line = line
	e.location.column = column

	return e
}

// Context specifies the context value. This is a message-
// dependent value that further describes the error. For
// example, in a keyword not recognized error, the context
// is usually the offending keyword.
func (e *EgoErrorMsg) Context(context interface{}) *EgoErrorMsg {
	if context != nil {
		e.context = fmt.Sprintf("%v", context)
	} else {
		e.context = "<nil>"
	}

	return e
}

// NewMessage create a new EgoError using an arbitrary
// string. This is used in cases where a fmt.Errorf() was
// used to generate an error string.
func NewMessage(m string) *EgoErrorMsg {
	return &EgoErrorMsg{
		err: errors.New(m),
	}
}

// Is compares the current error to the supplied error, and
// return a boolean indicating if they are the same.
func (e *EgoErrorMsg) Is(err error) bool {
	if e == nil {
		return false
	}

	return e.err == err
}

func Equals(e1, e2 error) bool {
	if e1 == nil && e2 == nil {
		return true
	}

	if e1 == nil || e2 == nil {
		return false
	}

	if e, ok := e1.(*EgoErrorMsg); ok {
		return e.Is(e2)
	}

	return e1 == e2
}

// Equal comparees an error to an arbitrary object. If the
// object is not an error, then the result is always false.
// If it is a native error or an EgoError, the error and
// wrapped error are compared.
func (e *EgoErrorMsg) Equal(v interface{}) bool {
	if e == nil {
		return v == nil
	}

	if v == nil {
		return Nil(e)
	}

	switch a := v.(type) {
	case *EgoErrorMsg:
		return e.err == a.err

	case error:
		return e.err == a

	default:
		return false
	}
}

// Nil tests to see if the error is "nil". If it is a native Go
// error, it is just tested to see if it is nil. If it is an
// EgoError then additionally we test to see if it is a valid
// pointer but to a null error, in which case it is also considered
// a nil value.
func Nil(e error) bool {
	if e == nil {
		return true
	}

	if ee, ok := e.(*EgoErrorMsg); ok {
		if ee == nil {
			return true
		}

		return ee.err == nil
	}

	return false
}

// Format an EgoError as a string for human consumption.
func (e *EgoErrorMsg) Error() string {
	var b strings.Builder

	if e == nil || e.err == nil {
		return ""
	}

	predicate := false

	// If we have a location, report that as module or module/line number
	if e.location != nil {
		if predicate {
			b.WriteString(", ")
		}

		if e.location.line > 0 {
			var lineStr string

			if e.location.column > 0 {
				lineStr = fmt.Sprintf("%d:%d", e.location.line, e.location.column)
			} else {
				lineStr = fmt.Sprintf("%d", e.location.line)
			}

			b.WriteString("at ")

			if len(e.location.name) > 0 {
				b.WriteString(fmt.Sprintf("%s(line %s)", e.location.name, lineStr))
			} else {
				b.WriteString(fmt.Sprintf("line %s", lineStr))
			}

			predicate = true
		} else {
			if e.location.name != "" {
				b.WriteString("in ")
				b.WriteString(e.location.name)
				predicate = true
			}
		}
	}

	// If we have an underlying error, report the string value for that
	if !Nil(e.err) {
		if !e.Is(ErrUserDefined) {
			if predicate {
				b.WriteString(", ")
			}

			b.WriteString(e.err.Error())

			predicate = true
		}
	}

	// If we have additional context, report that
	if e.context != "" {
		if predicate {
			b.WriteString(": ")
		}

		b.WriteString(e.context)
	}

	return b.String()
}

// Unwrap retrieves the native or wrapped error from this
// EgoError.
func (e *EgoErrorMsg) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.err
}

// GetContext retrieves the context value for the error.
func (e *EgoErrorMsg) GetContext() interface{} {
	if e == nil {
		return nil
	}

	return e.context
}
