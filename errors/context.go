package errors

import (
	"fmt"

	"github.com/tucats/ego/defs"
)

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
