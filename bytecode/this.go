package bytecode

import (
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
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
	var (
		found bool
		this  interface{}
	)

	// If the operand is a name, look up the value in the
	// symbol table. If it's not a name, then it's a value for
	// a receiver and we use it as-is.
	if name, ok := i.(string); ok {
		if len(name) == 0 {
			return c.runtimeError(errors.ErrInvalidIdentifier)
		}

		this, found = c.get(name)
		if !found {
			return c.runtimeError(errors.ErrUnknownIdentifier).Context(name)
		}
	} else {
		this = i
	}

	// Assign the value to a geenated name and put it on the reciver stack.
	_ = c.push(this)
	name := data.GenerateName()
	c.setAlways(name, this)

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
		c.symbols.MarkEphemeral(this)

		ui.Log(ui.TraceLogger, "trace.getthis", ui.A{
			"name":  this,
			"value": data.Format(v)})
	}

	return nil
}

// pushThis adds a receiver value to the "this" stack.
func (c *Context) pushThis(name string, v interface{}) {
	if c.receiverStack == nil {
		c.receiverStack = []this{}
	}

	c.receiverStack = append(c.receiverStack, this{name, v})
}

// popThis removes a receiver value from this "this" stack.
func (c *Context) popThis() (interface{}, bool) {
	if len(c.receiverStack) == 0 {
		return nil, false
	}

	this := c.receiverStack[len(c.receiverStack)-1]
	c.receiverStack = c.receiverStack[:len(c.receiverStack)-1]

	return this.value, true
}
