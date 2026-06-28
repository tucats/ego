package bytecode

import (
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
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

// explodeByteCode implements Explode. This accepts a *data.Map on the top of
// the stack, and creates local variables for each of the key-value pairs in
// the map.  The map must have string keys; non-string keys are rejected with
// ErrWrongMapKeyType.  After creating the variables, a bool is pushed
// indicating whether the map was empty (true = empty, false = had entries).
func explodeByteCode(c *Context, i any) error {
	var (
		err error
		v   any
	)

	v, err = c.Pop()
	if err != nil {
		return c.runtimeError(err)
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
