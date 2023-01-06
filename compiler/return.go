package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileReturn handles the return statement compilation.
func (c *Compiler) compileReturn() error {
	// Generate the deferal invocations, if any, in reverse order
	// that they were defined. Discard any results or stack leftovers.
	for i := len(c.deferQueue) - 1; i >= 0; i = i - 1 {
		dm := bytecode.NewStackMarker("defer")
		c.b.Emit(bytecode.Push, dm)
		c.b.Emit(bytecode.LocalCall, c.deferQueue[i])
		c.b.Emit(bytecode.DropToMarker, dm)
	}

	returnExpressions := []*bytecode.ByteCode{}
	hasReturnValue := false
	returnCount := 0

	for !c.isStatementEnd() {
		bc, err := c.Expression()
		if err != nil {
			return err
		}

		if returnCount >= len(c.coercions) {
			return c.newError(errors.ErrTooManyReturnValues)
		}

		bc.Append(c.coercions[returnCount])

		returnCount++

		hasReturnValue = true

		returnExpressions = append(returnExpressions, bc)

		if !c.t.IsNext(tokenizer.CommaToken) {
			break
		}
	}

	if returnCount < len(c.coercions) {
		return c.newError(errors.ErrMissingReturnValues)
	}

	// If there was a return value, the return values must be
	// pushed on the stack in reverse order so they match up
	// with any multiple-assignment calls.
	if hasReturnValue {
		// If there are multiple return values, start with pushing
		// a unique marker value.
		if len(returnExpressions) > 1 {
			c.b.Emit(bytecode.Push, bytecode.NewStackMarker(c.b.Name(), returnCount))
		}

		for i := len(returnExpressions) - 1; i >= 0; i = i - 1 {
			c.b.Append(returnExpressions[i])
		}
	}

	// Stop execution of this stream
	c.b.Emit(bytecode.Return, hasReturnValue)

	return nil
}
