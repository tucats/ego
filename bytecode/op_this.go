package bytecode

import (
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// SetThisImpl implements the SetThis opcode. Given a named value,
// the current value is pushed on the "this" stack as part of setting
// up a call, to be retrieved later by the body of the call.
func SetThisImpl(c *Context, i interface{}) *errors.EgoError {
	if v, ok := c.Get(util.GetString(i)); ok {
		c.PushThis(v)
	}

	return nil
}

// GetThisImpl implements the GetThis opcode. Given a value name,
// get the top-most item from the "this" stack and store it in the
// named value. This is done as part of prologue of a function that
// has a receiver.
func GetThisImpl(c *Context, i interface{}) *errors.EgoError {
	this := util.GetString(i)

	if v, ok := c.PopThis(); ok {
		return c.SetAlways(this, v)
	}

	return nil
}

// PushThis adds a receiver value to the "this" stack.
func (c *Context) PushThis(v interface{}) {
	if c.thisStack == nil {
		c.thisStack = []interface{}{}
	}

	c.thisStack = append(c.thisStack, v)
}

// PopThis removes a receiver value from this "this" stack.
func (c *Context) PopThis() (interface{}, bool) {
	if c.thisStack == nil || len(c.thisStack) == 0 {
		return nil, false
	}

	v := c.thisStack[len(c.thisStack)-1]
	c.thisStack = c.thisStack[:len(c.thisStack)-1]

	return v, true
}
