package bytecode

import (
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
)

// loadByteCode instruction processor. This function loads a value
// from the symbol table and pushes it onto the stack.
func loadByteCode(c *Context, i any) error {
	name := data.String(i)
	if len(name) == 0 {
		return c.runtimeError(errors.ErrInvalidIdentifier).Context(name)
	}

	v, found := c.get(name)
	if !found {
		return c.runtimeError(errors.ErrUnknownIdentifier).Context(name)
	}

	return c.push(data.UnwrapConstant(v))
}
