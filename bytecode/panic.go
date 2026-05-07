package bytecode

import (
	"fmt"

	"github.com/tucats/ego/errors"
)

// userPanicByteCode implements the UserPanic opcode, which is emitted for the
// Ego panic() built-in. Unlike the Panic opcode (used by @fail and fatal
// conditions), UserPanic starts a recoverable unwind: it sets the panic state
// on the context and returns ErrPanicActive so that the run loop can drive the
// frame-by-frame unwind via unwindPanic().
//
// If an operand is provided it is used as the panic value; otherwise the value
// is popped from the stack.
func userPanicByteCode(c *Context, i any) error {
	var panicValue any

	if i != nil {
		panicValue = i
	} else {
		v, err := c.Pop()
		if err != nil {
			return err
		}

		panicValue = v
	}

	c.panicActive = true
	c.panicValue = panicValue

	return errors.ErrPanicActive
}

// recoverByteCode implements the Recover opcode. recover() is only meaningful
// inside a deferred function that was invoked during panic unwinding. It walks
// the panicContext chain from the current (child) context back to the context
// that is actively panicking, clears the panic state there, and pushes the
// panic value onto the current stack. If no panic is in progress it pushes nil.
func recoverByteCode(c *Context, i any) error {
	// Walk the panicContext chain to find a context that is actively panicking.
	panicking := findPanickingContext(c)

	if panicking == nil {
		// Not called during panic unwinding: recover() returns nil.
		return c.push(nil)
	}

	// Retrieve and clear the panic state on the panicking context.
	value := panicking.panicValue
	panicking.panicActive = false
	panicking.panicValue = nil

	// Push the panic value so the caller can inspect it.
	return c.push(value)
}

// findPanickingContext walks the panicContext chain starting from ctx and returns
// the first context that has panicActive == true. Returns nil if not found.
func findPanickingContext(ctx *Context) *Context {
	for ctx != nil {
		if ctx.panicActive {
			return ctx
		}

		ctx = ctx.panicContext
	}

	return nil
}

// unwindPanic is called by the run loop when it receives ErrPanicActive. It
// drives the frame-by-frame unwinding: for each frame on the call stack it
// runs any deferred functions (giving them a chance to call recover()), then
// pops the frame.
//
// If a deferred function calls recover() and clears the panic state, unwindPanic
// clears the defer stack (so it is not re-run by a later RunDefers opcode), pops
// the current call frame, and returns nil so the run loop resumes in the caller
// of the panicking function. Otherwise, once all frames are exhausted, it prints
// a fatal panic message and returns ErrStop.
func (c *Context) unwindPanic() error {
	for {
		// Run the deferred functions for the current frame; any of them may call
		// recover() to clear panicActive on the context.
		if len(c.deferStack) > 0 {
			if err := c.invokePanicDefers(); err != nil {
				return err
			}
		}

		if !c.panicActive {
			// A deferred function called recover(). Clear the defer stack so that
			// the RunDefers opcode (which is about to execute in the resumed
			// function) does not re-run them.
			c.deferStack = []deferStatement{}

			// Pop the current call frame so execution resumes in the caller of
			// the panicking function rather than continuing inside it.
			if c.framePointer == 0 {
				// No call frame to pop — we are at the top level. Stop normally.
				c.running.Store(false)

				return errors.ErrStop
			}

			// Discard locals/partial results, then restore the caller's context.
			c.stackPointer = c.framePointer

			return c.callFramePop()
		}

		// No recovery happened in this frame. Pop the frame and try the caller.
		if c.framePointer == 0 {
			break
		}

		// Discard anything above the frame pointer (locals, partial results).
		c.stackPointer = c.framePointer

		if err := c.callFramePop(); err != nil {
			break
		}
	}

	// Panic reached the top of the call stack with no recover() — fatal.
	c.running.Store(false)

	panicMessage := fmt.Sprintf("%v", c.panicValue)
	c.panicActive = false
	c.panicValue = nil

	fmt.Fprintf(c.output, "panic: %s\n", panicMessage)
	fmt.Fprint(c.output, c.FormatFrames(IncludeSymbolTableNames))

	return errors.ErrStop
}
