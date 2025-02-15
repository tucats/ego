package bytecode

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
)

// runtimeError is a helper function used to generate a new runtimeError based
// on the runtime context. The current module name and line number
// from the context are stored in the new runtimeError object, along with
// the message and context.
func (c *Context) runtimeError(err error, context ...interface{}) *errors.Error {
	if err == nil {
		return nil
	}

	var r *errors.Error

	// If this is already an error, make a clone of it and add the module and location
	// info and return it.
	if e, ok := err.(*errors.Error); ok {
		e = e.Clone()

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

// Implement the Signal bytecode, which generates an arbitrary error return,
// using the instruction opcode.
func signalByteCode(c *Context, i interface{}) error {
	if i == nil {
		if v, err := c.Pop(); err != nil {
			return err
		} else {
			i = v
		}
	}

	if e, ok := i.(*errors.Error); ok {
		return c.runtimeError(e)
	}

	if e, ok := i.(error); ok {
		return c.runtimeError(errors.New(e))
	}

	return c.runtimeError(errors.Message(data.String(i)))
}
