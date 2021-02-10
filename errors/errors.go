package errors

import (
	"fmt"
	"strings"
)

type location struct {
	name string
	line int
}

type EgoError struct {
	err      error
	location *location
	context  string
}

func New(err error) *EgoError {
	if e, ok := err.(*EgoError); ok {
		return e
	}

	return &EgoError{
		err: err,
	}
}

func (e *EgoError) In(name string) *EgoError {
	return e.At(name, 0)
}

func (e *EgoError) At(name string, line int) *EgoError {
	e.location = &location{
		name: name,
		line: line,
	}

	return e
}

func (e *EgoError) WithContext(context interface{}) *EgoError {
	e.context = fmt.Sprintf("%v", context)

	return e
}

func NewMessage(m string) *EgoError {
	return &EgoError{
		context: m,
	}
}

func (e *EgoError) Is(err error) bool {
	if e == nil {
		return false
	}

	return e.err == err
}

func (e *EgoError) Error() string {
	var b strings.Builder

	predicate := false

	// If we have a location, report that as module or module/line number
	if e.location != nil {
		if predicate {
			b.WriteString(", ")
		}

		if e.location.line > 0 {
			b.WriteString("at ")

			if len(e.location.name) > 0 {
				b.WriteString(fmt.Sprintf("%s(line %d)", e.location.name, e.location.line))
			} else {
				b.WriteString(fmt.Sprintf("line %d", e.location.line))
			}
		} else {
			b.WriteString("in ")
			b.WriteString(e.location.name)
		}

		predicate = true
	}

	// If we have an underlying error, report the string value for that
	if e.err != nil {
		if predicate {
			b.WriteString(", ")
		}

		b.WriteString(e.err.Error())

		predicate = true
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

func (e *EgoError) Unwrap() error {
	return e.err
}
