package bytecode

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
)

// entryPointByteCode instruction processor calls a function as the main
// program of this Ego invocation The name can be an operand to the
// function, or named in the string on the top of the stack.
func entryPointByteCode(c *Context, i interface{}) error {
	var entryPointName string

	if i != nil {
		entryPointName = data.String(i)
	} else {
		v, _ := c.Pop()
		entryPointName = data.String(v)
	}

	if entryPoint, found := c.get(entryPointName); found {
		_ = c.push(entryPoint)

		return callByteCode(c, 0)
	}

	return c.error(errors.ErrUndefinedEntrypoint).Context(entryPointName)
}
