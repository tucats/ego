package bytecode

import (
	"text/template"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
)

/******************************************\
*                                         *
*           T E M P L A T E S             *
*                                         *
\******************************************/

// templateByteCode compiles a template string from the stack and stores it in
// the template manager for the execution context.
func templateByteCode(c *Context, i interface{}) error {
	name := data.String(i)

	t, err := c.Pop()
	if err == nil {
		if isStackMarker(t) {
			return c.error(errors.ErrFunctionReturnedVoid)
		}

		t, e2 := template.New(name).Parse(data.String(t))
		if e2 == nil {
			err = c.push(t)
		} else {
			err = c.error(e2)
		}
	}

	return err
}
