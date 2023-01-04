package bytecode

import (
	"github.com/tucats/ego/errors"
)

// newError is a helper function used to generate a new error based
// on the runtime context. The current module name and line number
// from the context are stored in the new error object, along with
// the message and context.
func (c *Context) newError(err error, context ...interface{}) *errors.EgoErrorMsg {
	if err == nil {
		return nil
	}

	r := errors.EgoError(err).In(c.Name).At(c.GetLine(), 0)

	if len(context) > 0 {
		r = r.Context(context[0])
	}

	return r
}
