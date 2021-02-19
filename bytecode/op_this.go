package bytecode

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/util"
)

// SetThisImpl implements the SetThis opcode. Given a named value,
// the current value is pushed on the "this" stack as part of setting
// up a call, to be retrieved later by the body of the call.
func SetThisImpl(c *Context, i interface{}) *errors.EgoError {
	name := util.GetString(i)
	if v, ok := c.Get(name); ok {
		c.PushThis(name, v)
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
func (c *Context) PushThis(name string, v interface{}) {
	if c.thisStack == nil {
		c.thisStack = []This{}
	}

	c.thisStack = append(c.thisStack, This{name, v})
	c.PrintThisStack("after push")
}

// PopThis removes a receiver value from this "this" stack.
func (c *Context) PopThis() (interface{}, bool) {
	c.PrintThisStack("before pop")

	if c.thisStack == nil || len(c.thisStack) == 0 {
		return nil, false
	}

	this := c.thisStack[len(c.thisStack)-1]
	c.thisStack = c.thisStack[:len(c.thisStack)-1]

	return this.value, true
}

func (c Context) PrintThisStack(operation string) {
	if v, ok := ui.Loggers[ui.ByteCodeLogger]; v && ok {
		var b strings.Builder

		label := "@ " + operation + "; stack ="

		if c.thisStack == nil || len(c.thisStack) == 0 {
			b.WriteString(fmt.Sprintf("%13s%s %v", " ", label, "<empty>"))
		} else {
			b.WriteString(fmt.Sprintf("%13s%s ", " ", label))
			for index, v := range c.thisStack {
				if index > 0 {
					b.WriteString(", ")
				}

				n := v.name
				r, _ := functions.Type(c.symbols, []interface{}{v.value})
				b.WriteString(fmt.Sprintf("%s(%s)", n, r))
			}
		}

		ui.Debug(ui.ByteCodeLogger, "%s", b.String())
	}
}
