package bytecode

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
)

func storeBytecodeByteCode(c *Context, i interface{}) error {
	var (
		err error
		v   interface{}
	)

	if v, err = c.Pop(); err == nil {
		if isStackMarker(v) {
			return c.error(errors.ErrFunctionReturnedVoid)
		}

		if bc, ok := v.(*ByteCode); ok {
			bc.name = data.String(i)
			c.symbols.SetAlways(bc.name, bc)
		} else {
			return c.error(errors.ErrInvalidType).Context(data.TypeOf(v).String())
		}
	}

	return err
}
