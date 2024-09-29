package bytecode

import (
	"github.com/tucats/ego/errors"
)

// error is a helper function used to generate a new error based
// on the runtime context. The current module name and line number
// from the context are stored in the new error object, along with
// the message and context.
func (c *Context) error(err error, context ...interface{}) *errors.Error {
	if err == nil {
		return nil
	}

	var r *errors.Error

	// If this is already an error, just add the module and location
	// info and return it.
	if e, ok := err.(*errors.Error); ok {
		if !e.HasIn() {
			if c.module != "" {
				e = e.In(c.module)
			} else if c.name != "" {
				e = e.In(c.name)
			}
		}

		if !e.HasAt() {
			e = e.At(c.GetLine(), 0)
		}

		r = e
	} else {
		// Construct a new error with the current module name and line number.
		r = errors.New(err).In(c.name).At(c.GetLine(), 0)
	}

	if len(context) > 0 {
		r = r.Context(context[0])
	}

	return r
}
