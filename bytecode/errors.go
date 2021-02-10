package bytecode

import (
	"github.com/tucats/ego/errors"
)

// NewError is a helper function used to generate a new error based
// on the runtime context. The current module name and line number
// from the context are stored in the new error object, along with
// the message and context.
func (c *Context) NewError(err error, context ...interface{}) *errors.EgoError {
	if err == nil {
		return nil
	}

	r := errors.New(err).At(c.Name, c.GetLine())

	if len(context) > 0 {
		_ = r.WithContext(context[0])
	}

	return r
}
