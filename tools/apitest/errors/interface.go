package errors

import (
	"fmt"
	"strings"
)

type Error struct {
	Message    string
	Additional string
	Next       *Error
}

func (e *Error) Error() string {
	more := ""
	if e.Next != nil {
		more = e.Next.Error() + ", "
	}

	if e.Additional == "" {
		return more + e.Message
	}

	return more + fmt.Sprintf("%s: %v", e.Message, e.Additional)
}

func New(message any) *Error {
	switch actual := message.(type) {
	case string:
		return &Error{Message: actual}

	case *Error:
		return actual.Clone()
	}

	return New(fmt.Sprintf("%v", message))
}

func (e *Error) Clone() *Error {
	if e.Next == nil {
		return e
	}

	return &Error{
		Message:    e.Message,
		Additional: e.Additional,
		Next:       e.Next,
	}
}

func (e *Error) Chain(err *Error) *Error {
	if err == nil {
		return e
	}

	return &Error{
		Message:    e.Message,
		Additional: e.Additional,
		Next:       err,
	}
}

func (e *Error) Context(context any) *Error {
	if e == nil {
		return nil
	}

	switch actual := context.(type) {
	case []string:
		e.Additional = strings.Join(actual, ", ")

	default:
		e.Additional = fmt.Sprintf("%v", context)
	}

	return e
}
