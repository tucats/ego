package bytecode

import (
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

// handleCatch processes any try/catch state that is in effect after an instruction
// executes. If there is no error, there is no action. If there is an error, then this
// code determines if the error qualifies to be considered "caught" and if so will
// redirect execution to a new instruction (and return nil indicating there is no
// longer an error condition). If no try/catch is active, or it specifies error(s)
// different than the one found, then it simply returns the error for further processing
// in the main run loop.
func handleCatch(c *Context, err error) error {
	// If there is no error, we're done.
	if err == nil || errors.Equals(err, errors.ErrStop) {
		return nil
	}

	text := err.Error()

	// See if we are in a try/catch block. If there is a Try/Catch stack
	// and the jump point on top is non-zero, then we can transfer control.
	// Note that if the error was fatal, the running flag is turned off, which
	// prevents the try block from being honored (i.e. you cannot catch a fatal
	// error).
	if len(c.tryStack) > 0 && c.tryStack[len(c.tryStack)-1].addr > 0 && c.running {
		// Do we have a selective set of things we catch? The default is that we
		// catch everything, but if the try info block has a list of errors, then
		// we only catch if the error is on that specific list.
		willCatch := true

		try := c.tryStack[len(c.tryStack)-1]
		if len(try.catches) > 0 {
			willCatch = false

			for _, e := range try.catches {
				if e.(*errors.Error).Equal(err) {
					willCatch = true

					break
				}
			}
		}

		// If we aren't catching it, just percolate the error
		if !willCatch {
			return errors.New(err)
		}

		// This could be in a function call tree within the try stack. So drop the items on the stack until
		// we get to the try marker. If, along the way, we find a stack frame, then pop the stack frame as well
		// so we reset the state of the context back to the frame level where the try was initiated.

		for {
			v, err := c.Pop()
			if err != nil {
				return err
			}

			// If its a call frame, put it back on the stack and then do the formal
			// pop of a call frame, which updates the state of the context.
			if f, ok := v.(*CallFrame); ok {
				ui.Log(ui.TraceLogger, "trace.unwind", ui.A{
					"thread": c.threadID,
					"module": f.Module,
					"line":   f.Line,
					"frame":  f.symbols.Name})

				if err := c.push(v); err != nil {
					return err
				}

				if err := c.callFramePop(); err != nil {
					return err
				}

				continue
			}

			// See if we've hit the frame marker that ends the main try{} block. If so we're done.
			if isStackMarker(v, "try") {
				break
			}
		}

		// We are catching, so update the PC
		c.programCounter = try.addr

		// Zero out the jump point for this try/catch block so recursive
		// errors don't occur.
		c.tryStack[len(c.tryStack)-1].addr = 0

		// Record the error in the "__error" variable for use in the catch block if needed.
		c.symbols.SetAlways(defs.ErrorVariable, err)

		if ui.IsActive(ui.TraceLogger) {
			ui.Log(ui.TraceLogger, "trace.branch.catch", ui.A{
				"thread": c.threadID,
				"addr":   c.programCounter,
				"error":  text})
		}

		// Successfully redirected to a catch block, so no more error state.
		return nil
	}

	// No catch, so error still active.
	return err
}
