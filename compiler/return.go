package compiler

import (
	"fmt"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
)

// Return handles the return statement compilation.
func (c *Compiler) Return() *errors.EgoError {
	// Generate the deferal invocations, if any, in reverse order
	// that they were defined.
	for i := len(c.deferQueue) - 1; i >= 0; i = i - 1 {
		c.b.Emit(bytecode.LocalCall, c.deferQueue[i])
	}

	returnExpressions := []*bytecode.ByteCode{}
	hasReturnValue := false
	returnCount := 0

	for !c.StatementEnd() {
		bc, err := c.Expression()
		if !errors.Nil(err) {
			return err
		}

		if returnCount >= len(c.coercions) {
			return c.NewError(errors.TooManyReturnValues)
		}

		bc.Append(c.coercions[returnCount])

		returnCount++

		hasReturnValue = true

		returnExpressions = append(returnExpressions, bc)

		if !c.t.IsNext(",") {
			break
		}
	}

	if returnCount < len(c.coercions) {
		return c.NewError(errors.MissingReturnValues)
	}

	// If there was a return value, the return values must be
	// pushed on the stack in reverse order so they match up
	// with any multiple-assignment calls.
	if hasReturnValue {
		// If there are multiple return values, start with pushing
		// a unique marker value.
		if len(returnExpressions) > 1 {
			c.b.Emit(bytecode.Push, bytecode.StackMarker{
				Desc: fmt.Sprintf("%s(),%d]", c.b.Name, returnCount),
			})
		}

		for i := len(returnExpressions) - 1; i >= 0; i = i - 1 {
			c.b.Append(returnExpressions[i])
		}
	}

	// Stop execution of this stream
	c.b.Emit(bytecode.Return, hasReturnValue)

	return nil
}

// Exit handles the exit statement compilation.
func (c *Compiler) Exit() *errors.EgoError {
	c.b.Emit(bytecode.Load, "util")
	c.b.Emit(bytecode.Member, "Exit")

	argCount := 0

	if !c.StatementEnd() {
		bc, err := c.Expression()
		if !errors.Nil(err) {
			return err
		}

		c.b.Append(bc)

		argCount = 1
	}

	c.b.Emit(bytecode.Call, argCount)

	return nil
}
