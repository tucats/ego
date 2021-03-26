package bytecode

import (
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

func setRegisterByteCode(c *Context, i interface{}) *errors.EgoError {
	idx := util.GetInt(i)
	if idx < 0 || idx >= registerCount {
		return c.newError(errors.RegisterAddressError)
	}

	v, err := c.Pop()
	if err != nil {
		return err
	}

	c.registers[idx] = v

	return nil
}

func getRegisterByteCode(c *Context, i interface{}) *errors.EgoError {
	idx := util.GetInt(i)
	if idx < 0 || idx >= registerCount {
		return c.newError(errors.RegisterAddressError)
	}

	return c.stackPush(c.registers[idx])
}
