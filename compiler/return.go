package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileReturn compiles a return statement. The "return" keyword has already
// been consumed by the caller.
//
// Four cases are handled:
//
//  1. Named return variables, bare return: RunDefers is emitted, then the
//     named variables are loaded from the symbol table and pushed, then
//     Return is emitted with the count.
//
//  2. Named return variables, explicit value(s): each expression is compiled,
//     coerced, and stored into the corresponding named return variable. Then
//     RunDefers is emitted (so deferred functions see the post-assignment
//     values), then the named variables are loaded and Return is emitted.
//
//  3. Explicit return expressions (no named variables): each expression is
//     compiled, coerced, and collected in order. If more than one value is
//     returned, a stack marker is pushed first. Expressions are pushed in
//     reverse order and Return is emitted with the count.
//
//  4. Bare return (no named variables, no expression): Return is emitted
//     with a count of 0.
func (c *Compiler) compileReturn() error {
	// Do we have named return values?
	if len(c.returnVariables) > 0 {
		// If explicit return values are provided, assign them to the named
		// return variables before running defers. This matches Go semantics:
		// the assignment happens first, then deferred functions run (and may
		// observe or modify the named variables), then the final values are
		// returned.
		if !c.isStatementEnd() {
			count := 0

			for !c.isStatementEnd() {
				if count >= len(c.returnVariables) {
					return c.compileError(errors.ErrReturnValueCount)
				}

				bc, err := c.Expression(true)
				if err != nil {
					return err
				}

				if count < len(c.coercions) {
					bc.Append(c.coercions[count])
				}

				c.b.Append(bc)
				c.b.Emit(bytecode.Store, c.returnVariables[count].Name)

				count++

				if !c.t.IsNext(tokenizer.CommaToken) {
					break
				}
			}

			if count < len(c.returnVariables) {
				return c.compileError(errors.ErrMissingReturnValues)
			}
		}

		// RunDefers runs after explicit assignments so deferred functions
		// observe the updated named-variable values.
		c.b.Emit(bytecode.RunDefers)

		c.b.Emit(bytecode.Push, bytecode.NewStackMarker(c.b.Name(), len(c.returnVariables)))

		// Push the return values on the stack in the reverse order they
		// were declared.
		for i := len(c.returnVariables) - 1; i >= 0; i = i - 1 {
			if err := c.ReferenceSymbol(c.returnVariables[i].Name); err != nil {
				return err
			}

			c.b.Emit(bytecode.Load, c.returnVariables[i].Name)
		}

		c.b.Emit(bytecode.Return, len(c.returnVariables))

		return nil
	}

	// If there are deferred statements stored in the runtime context, run them.
	c.b.Emit(bytecode.RunDefers)

	// Start processing return expressions (there can be multiple
	// return values).
	returnExpressions := []*bytecode.ByteCode{}
	hasReturnValue := false
	returnCount := 0

	for !c.isStatementEnd() {
		bc, err := c.Expression(true)
		if err != nil {
			return err
		}

		if returnCount >= len(c.coercions) {
			return c.compileError(errors.ErrReturnValueCount)
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
		return c.compileError(errors.ErrMissingReturnValues)
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

	// Stop execution of this stream. Let the caller know how many variables
	// we are returning.
	c.b.Emit(bytecode.Return, returnCount)

	return nil
}
