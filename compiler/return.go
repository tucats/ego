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
	_ = c.deferInvocations()

	// Start processing return expressions (there can be multiple
	// return values).
	returnExpressions := []*bytecode.ByteCode{}
	hasReturnValue := false
	returnCount := 0

	for !c.isStatementEnd() {
		bc, err := c.Expression()
		if err != nil {
			return err
		}

		if returnCount >= len(c.coercions) {
			return c.error(errors.ErrTooManyReturnValues)
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
		return c.error(errors.ErrMissingReturnValues)
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

	// Mark the compiler as having seen an explicit return.
	c.flags.returnLastStatement = true

	return nil
}

// We are exiting from a function, so generate the defer statement
// invocations.
func (c *Compiler) deferInvocations() error {
	for i := len(c.deferQueue) - 1; i >= 0; i = i - 1 {
		// Generate code to test if this one has been executed. If not,
		// skip it.
		c.b.Emit(bytecode.CheckDefer, c.deferQueue[i].Name)
		deferTestAddress := c.b.Mark()
		c.b.Emit(bytecode.BranchFalse, 0)

		// Generate code to indicate this one has been executed. This prevents
		// unwanted recursion at the end of the function body.
		c.b.Emit(bytecode.Push, false)
		c.b.Emit(bytecode.StoreGlobal, c.deferQueue[i].Name)

		// Call the statement. A stack marker is used to clean up the stack after
		// each statement executes.
		dm := bytecode.NewStackMarker("defer")
		c.b.Emit(bytecode.Push, dm)
		c.b.Emit(bytecode.LocalCall, c.deferQueue[i].Address)
		c.b.Emit(bytecode.DropToMarker, dm)

		// Store the forward reference in the test code to skip this
		// defer statement if it has already been executed.
		if err := c.b.SetAddressHere(deferTestAddress); err != nil {
			return err
		}
	}

	return nil
}
