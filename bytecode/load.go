package bytecode

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
)

// loadByteCode instruction processor. This function loads a value
// from the symbol table and pushes it onto the stack.
func loadByteCode(c *Context, i interface{}) error {
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

// explodeByteCode implements Explode. This accepts a struct on the top of
// the stack, and creates local variables for each of the members of the
// struct by their name.
func explodeByteCode(c *Context, i interface{}) error {
	var (
		err error
		v   interface{}
	)

	v, err = c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(v) {
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	empty := true

	if m, ok := v.(*data.Map); ok {
		if !m.KeyType().IsString() {
			err = c.runtimeError(errors.ErrWrongMapKeyType)
		} else {
			keys := m.Keys()

			for _, k := range keys {
				empty = false
				v, _, _ := m.Get(k)

				c.setAlways(data.String(k), v)
			}

			return c.push(empty)
		}
	} else {
		err = c.runtimeError(errors.ErrInvalidType).Context(data.TypeOf(v).String())
	}

	return err
}
