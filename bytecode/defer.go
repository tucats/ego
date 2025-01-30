package bytecode

import (
	"strconv"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
)

func deferStartByteCode(c *Context, i interface{}) error {
	c.deferThisSize = len(c.receiverStack)

	// If asked to capture current scope, do so now.
	b, err := data.Bool(i)
	if err != nil {
		return c.error(err)
	}

	if b {
		c.deferSymbols = c.symbols
	} else {
		c.deferSymbols = nil
	}

	return nil
}

// deferByteCode is the bytecode for the defer statement. This captures
// the arguments to the function as well as the function target object,
// and stores them in the runtime context. This is then executed when
// a return statement or end-of-funcction boundary is processed.
func deferByteCode(c *Context, i interface{}) error {
	argc, err := data.Int(i)
	args := []interface{}{}
	name := c.GetModuleName() + ":" + strconv.Itoa(c.GetLine())

	if err != nil {
		return c.error(err)
	}

	// Get the arguments to the function
	for j := 0; j < argc; j++ {
		if arg, err := c.Pop(); err == nil {
			args = append(args, arg)
		} else {
			return err
		}
	}

	// Get the function to execute
	f, err := c.Pop()
	if err != nil {
		return err
	}

	var receivers []this

	// If at the start of the defer we captured the size of the receiver stack
	// and the receiver stack size is now larger, it means the defer exprssion
	// included receiver values. We need to capture these values and store them
	// in the defer stack object.
	if c.deferThisSize > 0 && (c.deferThisSize < len(c.receiverStack)) {
		// Capture the slide of the this stack since we started the defer.
		receivers = c.receiverStack[len(c.receiverStack)-c.deferThisSize:]
		c.receiverStack = c.receiverStack[:c.deferThisSize]
	}

	// Push the function and arguments onto the defer stack
	c.deferStack = append(c.deferStack, deferStatement{
		target:        f,
		receiverStack: receivers,
		args:          args,
		name:          name,
		symbols:       c.deferSymbols,
	})

	return nil
}

// runDefersByteCode is the bytecode for the run-defers statement.
func runDefersByteCode(c *Context, i interface{}) error {
	if len(c.deferStack) > 0 {
		return c.invokeDeferredStatements()
	}

	return nil
}

// invokeDeferredStatements executes the deferred statements in the
// reverse order they were defined. This is called when a return
// is executed, or the function completes execution.
func (c *Context) invokeDeferredStatements() error {
	// Run the deferred statements in reverse order.
	for i := len(c.deferStack) - 1; i >= 0; i-- {
		deferTask := c.deferStack[i]

		// Create a new bytecode area to execute the defer operations. It is
		// always marked as literal (which maeans it cannot be a scope boundary)
		// because the fragment will either call a function literal, or it's a
		// simple call to a function that sets it's own boundary scope.
		cb := New("defer " + deferTask.name).Literal(true)

		// Push the target function onto the stack
		cb.Emit(Push, deferTask.target)

		// Push the arguments in reverse order from the stack
		for j := len(deferTask.args) - 1; j >= 0; j-- {
			cb.Emit(Push, deferTask.args[j])
		}

		cb.Emit(Call, len(deferTask.args))

		s := c.symbols
		if deferTask.symbols != nil {
			s = deferTask.symbols.Boundary(false)

			ui.Log(ui.SymbolLogger, "symbols.defer.task", ui.A{
				"name": s.Name,
				"id":   s.ID()})
		}

		// Create a context for executing the deferred statement. The context
		// is given the active receiver stack associated with the defer statement.
		cx := NewContext(s, cb)
		cx.receiverStack = deferTask.receiverStack

		// Execute the deferred statement
		if err := cx.Run(); err != nil && err != errors.ErrStop {
			return err
		}
	}

	return nil
}
