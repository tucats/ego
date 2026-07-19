package bytecode

import (
	"strconv"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
)

// deferSTart is the bytecode that starts a deferred statement declaration. It
// captures the state of the symbol table at the time of the defer operation, so
// any symbols in scope at that time are available to the defer statement or
// function call.
func deferStartByteCode(c *Context, i any) error {
	c.deferThisSize = len(c.receiverStack)

	// If asked to capture current scope, do so now.
	b, err := data.Bool(i)
	if err != nil {
		return c.runtimeError(err)
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
// a return statement or end-of-function boundary is processed.
func deferByteCode(c *Context, i any) error {
	argc, err := data.Int(i)
	args := []any{}
	name := c.GetModuleName() + ":" + strconv.Itoa(c.GetLine())

	if err != nil {
		return c.runtimeError(err)
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
	// and the receiver stack size is now larger, it means the defer expression
	// included receiver values. We need to capture these values and store them
	// in the defer stack object.
	if c.deferThisSize > 0 && (c.deferThisSize < len(c.receiverStack)) {
		// Capture every receiver added SINCE deferStart — those start at index
		// deferThisSize.  The previous formula (len-deferThisSize) was wrong
		// when the number of new receivers differed from deferThisSize (DEFER-1).
		receivers = c.receiverStack[c.deferThisSize:]
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

// emitDeferredCall emits the Call instruction for a replayed deferred call
// (used by both invokeDeferredStatements and invokePanicDefers). Unlike a
// normally-compiled "X.Y(...)" call, this synthesized bytecode has no
// preceding SetThis of its own -- deferByteCode instead captured whatever
// receiver value(s) were already pending on the receiver stack at the time
// the defer statement was compiled and stashed them directly in
// deferTask.receiverStack (see deferByteCode above), and the caller has
// already seeded the replay context's receiver stack with them
// (cx.receiverStack = deferTask.receiverStack).
//
// callByteCode now only pops the receiver stack when the Call instruction's
// own operand says a receiver is pending (CALL-11) -- it no longer infers
// this from the callee's runtime type -- so this replay bytecode must signal
// that explicitly, exactly the way functionCall() does at normal compile
// time: use the two-element operand form when a receiver was captured, and
// the bare argument count otherwise.
func emitDeferredCall(cb *ByteCode, deferTask deferStatement) {
	if len(deferTask.receiverStack) > 0 {
		cb.Emit(Call, []any{len(deferTask.args), true})
	} else {
		cb.Emit(Call, len(deferTask.args))
	}
}

// runDefersByteCode is the bytecode for the run-defers statement.
func runDefersByteCode(c *Context, i any) error {
	if len(c.deferStack) > 0 {
		return c.invokeDeferredStatements()
	}

	return nil
}

// invokePanicDefers runs the deferred statements for thecurrent frame while
// a panic is in progress. It is identical to invokeDeferredStatements except
// that each child context has its panicContext field set to c so that a
// recover() call inside the deferred function can locate and clear the panic
// state on this (the panicking) context.
func (c *Context) invokePanicDefers() error {
	for i := len(c.deferStack) - 1; i >= 0; i-- {
		deferTask := c.deferStack[i]

		cb := New("defer " + deferTask.name).Literal(true)
		cb.Emit(Push, deferTask.target)

		for j := len(deferTask.args) - 1; j >= 0; j-- {
			cb.Emit(Push, deferTask.args[j])
		}

		// emitDeferredCall (see below) chooses the Call operand shape based on
		// whether deferTask.receiverStack has a value waiting to be consumed --
		// mirroring what functionCall() does at normal compile time (CALL-11).
		emitDeferredCall(cb, deferTask)

		s := c.symbols
		if deferTask.symbols != nil {
			s = deferTask.symbols.Boundary(false)
		}

		cx := NewContext(s, cb)
		cx.receiverStack = deferTask.receiverStack
		// Give the deferred child context a back-pointer to the panicking context
		// so that recover() (the Recover opcode) can walk back to find us.
		cx.panicContext = c

		if err := cx.Run(); err != nil && !errors.Equal(err, errors.ErrStop) {
			return err
		}
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
		// always marked as literal (which means it cannot be a scope boundary)
		// because the fragment will either call a function literal, or it's a
		// simple call to a function that sets it's own boundary scope.
		cb := New("defer " + deferTask.name).Literal(true)

		// Push the target function onto the stack
		cb.Emit(Push, deferTask.target)

		// Push the arguments in reverse order from the stack
		for j := len(deferTask.args) - 1; j >= 0; j-- {
			cb.Emit(Push, deferTask.args[j])
		}

		emitDeferredCall(cb, deferTask)

		s := c.symbols
		if deferTask.symbols != nil {
			s = deferTask.symbols.Boundary(false)

			if ui.IsActive(ui.SymbolLogger) {
				ui.Log(ui.SymbolLogger, "symbols.defer.task", ui.A{
					"name": s.Name,
					"id":   s.ID()})
			}
		}

		// Create a context for executing the deferred statement. The context
		// is given the active receiver stack associated with the defer statement.
		cx := NewContext(s, cb)
		cx.receiverStack = deferTask.receiverStack

		// Execute the deferred statement
		if err := cx.Run(); err != nil && !errors.Equal(err, errors.ErrStop) {
			return err
		}
	}

	return nil
}
