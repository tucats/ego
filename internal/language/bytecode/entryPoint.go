package bytecode

import (
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
)

// entryPointByteCode instruction processor calls a function as the main
// program of this Ego invocation The name can be an operand to the
// function, or named in the string on the top of the stack.
func entryPointByteCode(c *Context, i any) error {
	var entryPointName string

	if i != nil {
		entryPointName = data.String(i)
	} else {
		v, _ := c.Pop()
		entryPointName = data.String(v)
	}

	if entryPoint, found := c.get(entryPointName); found {
		_ = c.push(entryPoint)

		return callByteCode(c, 0)
	}

	return c.runtimeError(errors.ErrUndefinedEntrypoint).Context(entryPointName)
}

// entryPointExitByteCode runs immediately after the EntryPoint instruction's
// callee (e.g. "main") has returned control to the caller. If the entry
// point declared a return value (e.g. "func main() int"), that value is on
// the stack above the entrypointMarkerLabel marker pushed by the compiler;
// it becomes the process exit code, exactly as if os.Exit() had been called
// with that value. If the entry point declared no return value, nothing is
// found above the marker, and execution simply falls through to the natural
// end of the program (exit code 0).
func entryPointExitByteCode(c *Context, i any) error {
	var result any

	for {
		if c.stackPointer <= c.framePointer {
			break
		}

		v, err := c.Pop()
		if err != nil {
			break
		}

		if _, ok := v.(StackMarker); ok {
			break
		}

		result = v
	}

	if result != nil {
		return errors.ErrExit.Context(data.IntOrZero(result))
	}

	return nil
}
