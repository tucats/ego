package bytecode

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
)

// deferByteCode is the bytecode for the defer statement. This captures
// the arguments to the function as well as the function target object,
// and stores them in the runtime context. This is then executed when
// a return statement or end-of-funcction boundary is processed.
func deferByteCode(c *Context, i interface{}) error {
	argc := data.Int(i)
	args := []interface{}{}

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

	// Push the function and arguments onto the defer stack
	c.deferStack = append(c.deferStack, deferStatement{
		target: f,
		args:   args,
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
		ds := c.deferStack[i]

		// Create a new bytecode area to execute the defer operations.
		cb := New("defer")

		// Push the target function onto the stack
		cb.Emit(Push, ds.target)

		// Push the arguments in reverse order from the stack
		for j := len(ds.args) - 1; j >= 0; j-- {
			cb.Emit(Push, ds.args[j])
		}

		cb.Emit(Call, len(ds.args))

		// Create a context for executing the deferred statement
		cx := NewContext(c.symbols, cb)

		// Execute the deferred statement
		if err := cx.Run(); err != nil && err != errors.ErrStop {
			return err
		}
	}

	return nil
}
