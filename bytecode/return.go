package bytecode

import "github.com/tucats/ego/errors"

// returnByteCode implements the return opcode which returns from a called function
// or local subroutine.
func returnByteCode(c *Context, i interface{}) error {
	var err error

	// Do we have a return value?
	if b, ok := i.(bool); ok && b {
		c.result, err = c.Pop()
		if isStackMarker(c.Result) {
			return c.error(errors.ErrFunctionReturnedVoid)
		}
	} else if b, ok := i.(int); ok && b > 0 {
		// there are return items expected on the stack.
		if b == 1 {
			c.result, err = c.Pop()
		} else {
			c.result = nil
		}
	} else {
		// No return values, so flush any extra stuff left on stack.
		c.stackPointer = c.framePointer - 1
		c.result = nil
	}

	// If we are running in an active package table (such as running a non-receiver
	// function from the package) then hoist symbol table values from the package
	// symbol table back to the package object itself so they an be externally
	// referenced.
	if err := c.syncPackageSymbols(); err != nil {
		return errors.New(err)
	}

	// If FP is zero, there are no frames; this is a return from the main source
	// of the program or service.
	if c.framePointer > 0 {
		// Use the frame pointer to reset the stack and retrieve the
		// runtime state.
		err = c.callFramePop()
	} else {
		c.running = false
	}

	if err == nil && c.breakOnReturn {
		c.breakOnReturn = false

		return errors.ErrSignalDebugger
	}

	if err == nil {
		return err
	}

	return c.error(err)
}
