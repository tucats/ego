package bytecode

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/functions"
)

// setThisByteCode implements the SetThis opcode. Given a named value,
// the current value is pushed on the "this" stack as part of setting
// up a call, to be retrieved later by the body of the call. IF there
// is no name operand, assume the top stack value is to be used, and
// synthesize a name for it.
func setThisByteCode(c *Context, i interface{}) error {
	var name string

	if i == nil {
		v, err := c.Pop()
		if err != nil {
			return err
		}

		_ = c.push(v)
		name = data.GenerateName()
		c.setAlways(name, v)
	} else {
		name = data.String(i)
	}

	if v, ok := c.get(name); ok {
		c.pushThis(name, v)
	}

	return nil
}

// loadThisByteCode implements the LoadThis opcode. This combines the
// functionality of the Load followed by the SetThis opcodes.
func loadThisByteCode(c *Context, i interface{}) error {
	name := data.String(i)
	if len(name) == 0 {
		return c.error(errors.ErrInvalidIdentifier)
	}

	v, found := c.get(name)
	if !found {
		return c.error(errors.ErrUnknownIdentifier).Context(name)
	}

	_ = c.push(v)
	name = data.GenerateName()
	c.setAlways(name, v)

	if v, ok := c.get(name); ok {
		c.pushThis(name, v)
	}

	return nil
}

// getThisByteCode implements the GetThis opcode. Given a value name,
// get the top-most item from the "this" stack and store it in the
// named value. This is done as part of prologue of a function that
// has a receiver.
func getThisByteCode(c *Context, i interface{}) error {
	this := data.String(i)

	if v, ok := c.popThis(); ok {
		c.setAlways(this, v)
	}

	return nil
}

// pushThis adds a receiver value to the "this" stack.
func (c *Context) pushThis(name string, v interface{}) {
	if c.thisStack == nil {
		c.thisStack = []This{}
	}

	c.thisStack = append(c.thisStack, This{name, v})
	c.PrintThisStack("push")
}

// popThis removes a receiver value from this "this" stack.
func (c *Context) popThis() (interface{}, bool) {
	if c.thisStack == nil || len(c.thisStack) == 0 {
		return nil, false
	}

	this := c.thisStack[len(c.thisStack)-1]
	c.thisStack = c.thisStack[:len(c.thisStack)-1]

	c.PrintThisStack("pop")

	return this.value, true
}

// Add a line to the trace output that shows the "this" stack of
// saved function receivers.
func (c *Context) PrintThisStack(operation string) {
	if ui.IsActive(ui.TraceLogger) {
		var b strings.Builder

		label := fmt.Sprintf("(%d) %s this; stack =", c.threadID, operation)

		if c.thisStack == nil || len(c.thisStack) == 0 {
			b.WriteString(fmt.Sprintf("%s <empty>", label))
		} else {
			b.WriteString(fmt.Sprintf("%s ", label))

			lastOne := len(c.thisStack) - 1
			for index := lastOne; index >= 0; index-- {
				v := c.thisStack[index]
				n := v.name
				r, _ := functions.Type(c.symbols, []interface{}{v.value})
				b.WriteString(fmt.Sprintf("\"%s\" T(%s)", n, r))
				if index > 0 {
					b.WriteString(",")
				}
			}
		}

		ui.Debug(ui.SymbolLogger, b.String())
	}
}
