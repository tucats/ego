package errors

import (
	goerror "errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/i18n"
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
	location *location
	context  string
}

// New creates a new Error object, and fills in the native
// wrapped error. Note that if the value passed in is already
// an Error, then it is returned without re-wrapping it.
func New(err error) *Error {
	if err == nil {
		return nil
	}

	if e, ok := err.(*Error); ok {
		return e
	}

	return &Error{
		err: err,
	}
}

// In specifies the location name. This can be the name of
// a source code module, or a function name.
func (e *Error) In(name string) *Error {
	if e == nil {
		return nil
	}

	if e.location == nil {
		e.location = &location{}
	}

	e.location.name = name

	return e
}

// HasIn returns true if the associated error already has a module
// name in the location data.
func (e *Error) HasIn() bool {
	if e == nil || e.location == nil {
		return false
	}

	return e.location.name != ""
}

// At specifies a line number and column position related to
// the error. The line number is always present, the column
// is typically only set during compilation; if it is zero then
// it is not displayed.
func (e *Error) At(line int, column int) *Error {
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

// HasAt returns true if the  associated error has line and/or column info
// already in the location data.
func (e *Error) HasAt() bool {
	if e == nil || e.location == nil {
		return false
	}

	return e.location.line != 0 || e.location.column != 0
}

// Context specifies the context value. This is a message-
// dependent value that further describes the error. For
// example, in a keyword not recognized error, the context
// is usually the offending keyword.
func (e *Error) Context(context interface{}) *Error {
	if e == nil {
		return nil
	}

	if context != nil {
		e.context = fmt.Sprintf("%v", context)
	} else {
		e.context = defs.NilTypeString
	}

	return e
}

// Message creates a new Error using an arbitrary string.
// This is used in cases where a fmt.Errorf() was used to
// generate an error string.
//
// Note that the message text is first checked to see if it
// is an i18n error key. If so, the localized version of the
// error message is used. If the message starts with an "_"
// character, no i18n translation is performed.
func Message(m string) *Error {
	if strings.HasPrefix(m, defs.ReadonlyVariablePrefix) {
		m = m[1:]
	} else {
		m = i18n.E(m)
	}

	return &Error{
		err: goerror.New(m),
	}
}

// Is compares the current error to the supplied error, and
// return a boolean indicating if they are the same.
func (e *Error) Is(err error) bool {
	if e == nil && err == nil {
		return true
	}

	if e == nil || err == nil || e.err == nil {
		return false
	}

	// Is the test error one of the Ego "native" errors? If so
	// we need to compare both underlying error states.
	if e1, ok := err.(*Error); ok {
		if e1 == nil || e.err == nil || e1.err == nil {
			return false
		}

		return e.err.Error() == e1.err.Error()
	}

	// Otherwise, we're comparing against a Go native error, so
	// we compare our underlying error to the provided error.
	return e.err.Error() == err.Error()
}

// Similar to the Is() method, but this one compares any two error
// values without regard for whether they are Ego errors or not.
func Equals(e1, e2 error) bool {
	if e1 == nil && e2 == nil {
		return true
	}

	if e1 == nil || e2 == nil {
		return false
	}

	if e, ok := e1.(*Error); ok {
		return e.Is(e2)
	}

	return e1 == e2
}

// Equal comparees an error to an arbitrary object. If the
// object is not an error, then the result is always false.
// If it is a native error or an Ego Error, the error and
// wrapped error are compared.
func (e *Error) Equal(v interface{}) bool {
	if e == nil {
		return v == nil
	}

	if v == nil {
		return Nil(e)
	}

	switch a := v.(type) {
	case *Error:
		return e.err == a.err

	case error:
		return e.err == a

	default:
		return false
	}
}

// Nil tests to see if the error is "nil". If it is a native Go
// error, it is just tested to see if it is nil. If it is an
// Ego Error then additionally we test to see if it is a valid
// pointer but to a null error, in which case it is also considered
// a nil value.
func Nil(e error) bool {
	if e == nil {
		return true
	}

	if ee, ok := e.(*Error); ok {
		if ee == nil {
			return true
		}

		return ee.err == nil
	}

	return false
}

// Format an Ego Error as a string for human consumption.
func (e *Error) Error() string {
	var (
		b         strings.Builder
		predicate bool
	)

	if e == nil || e.err == nil {
		return ""
	}

	// If we have a location, report that as module or module/line number
	if e.location != nil {
		if predicate {
			b.WriteString(", ")
		}

		if e.location.line > 0 {
			lineStr := strconv.Itoa(e.location.line)

			if e.location.column > 0 {
				lineStr = lineStr + ":" + strconv.Itoa(e.location.column)
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

			msg := strings.TrimPrefix(e.err.Error(), "error.")
			predicate = true

			b.WriteString(msg)
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
// Error.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.err
}

// GetContext retrieves the context value for the error.
func (e *Error) GetContext() interface{} {
	if e == nil {
		return nil
	}

	return e.context
}

// GetFullContext retrieves the metadata value for the error.
func (e *Error) GetFullContext() map[string]interface{} {
	if e == nil {
		return nil
	}

	result := map[string]interface{}{
		"Module":  "",
		"Line":    0,
		"Context": "",
	}

	if e.location != nil {
		if e.location.name != "" {
			result["Module"] = e.location.name
		}

		if e.location.line > 0 {
			result["Line"] = e.location.line
		}
	}

	if e.context != "" {
		result["Context"] = e.context
	}

	return result
}
