package errors

import (
	"fmt"
	"strconv"
	"strings"
)

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

// Get the location portion of an Ego error as a string. Used mostly for formatting
// debug messages.
func (e *Error) GetLocation() string {
	var b strings.Builder

	// If we have a location, report that as module or module/line number
	if e.location != nil {
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
		} else {
			if e.location.name != "" {
				b.WriteString("in ")
				b.WriteString(e.location.name)
			}
		}
	}

	return b.String()
}
