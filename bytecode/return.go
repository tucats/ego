package bytecode

import "github.com/tucats/ego/errors"

// returnByteCode implements the return opcode which returns from a called function
// or local subroutine.
func returnByteCode(c *Context, i any) error {
	var err error

	// Do we have a return value?
	if b, ok := i.(bool); ok && b {
		c.result, err = c.Pop()
		if isStackMarker(c.Result) {
			return c.runtimeError(errors.ErrFunctionReturnedVoid)
		}

		c.resultSet = true
	} else if b, ok := i.(int); ok && b > 0 {
		if b == 1 {
			c.result, err = c.Pop()
			c.resultSet = true

			// Named-return functions push a StackMarker below the single return value
			// (non-named returns do not). If one is present, discard it so callFramePop
			// finds an empty topOfStackSlice and correctly uses the resultSet path.
			if err == nil && c.stackPointer > c.framePointer {
				if isStackMarker(c.stack[c.stackPointer-1]) {
					_, err = c.Pop()
				}
			}
		} else {
			c.result = nil
			c.resultSet = false
		}
	} else {
		// No return values, so flush any extra stuff left on stack.
		c.stackPointer = c.framePointer - 1
		c.result = nil
		c.resultSet = false
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
		c.running.Store(false)
	}

	if err == nil && c.breakOnReturn {
		c.breakOnReturn = false

		return errors.ErrSignalDebugger
	}

	if err == nil {
		return err
	}

	return c.runtimeError(err)
}
