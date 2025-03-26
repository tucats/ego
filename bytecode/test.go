package bytecode

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
)

// This is the name of the global variable that keeps track of
// the number of active tests.
const activeTestCountVariable = "__activeTests"

// pushTest increments a global value that counts if we have
// active tests in process. If there are no active tests, then
// the count is created.
func pushTestByteCode(c *Context, i interface{}) error {
	testCount := 1

	v, found := c.get(activeTestCountVariable)
	if found {
		testCount, _ = data.Int(v)
		testCount += 1
	}

	c.symbols.Root().SetAlways(activeTestCountVariable, testCount)

	return nil
}

// popTest decrements the global value that keeps track of the
// active tests. If the count drops to zero, then the global
// variable is deleted. Additionally, if the count drops to zero,
// the code branches to the address in the argument. This is used
// to skip past @PASS blocks when there are no active tests.
func popTestByteCode(c *Context, i interface{}) error {
	var (
		v     interface{}
		found bool
		count int
	)

	v, found = c.get(activeTestCountVariable)
	if found {
		count, _ = data.Int(v)
	}

	if count < 1 {
		// Delete the global variable that counts the tests.
		_ = c.symbols.Root().Delete(activeTestCountVariable, true)

		// Branch to the given address. If ti's out of bounds,
		// that's an error. If the address is a positive number,
		// branch to that address. Otherwise, stop execution
		destination, err := data.Int(i)
		if err != nil {
			return c.runtimeError(err)
		}

		if destination < 0 || destination > c.bc.Size() {
			return c.runtimeError(errors.ErrInvalidBytecodeAddress).Context(destination)
		}

		if destination > 0 {
			pc, err := data.Int(i)
			if err != nil {
				return c.runtimeError(err)
			}

			c.SetPC(pc)
		} else {
			return errors.ErrStop
		}
	} else {
		c.symbols.Root().SetAlways(activeTestCountVariable, count-1)
	}

	return nil
}
