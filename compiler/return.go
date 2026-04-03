package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileReturn compiles a return statement. The "return" keyword has already
// been consumed by the caller.
//
// Before emitting the actual Return instruction, a RunDefers instruction is
// always emitted so that any pending defer'd function calls are executed on
// the way out.
//
// Three cases are handled:
//
//  1. Named return variables: if the function was declared with named return
//     values (e.g. "func f() (x int, err error)"), those variables are loaded
//     from the symbol table and pushed onto the stack, then Return is emitted
//     with the count of named variables.
//
//  2. Explicit return expressions: each expression is compiled, run through
//     the appropriate type-coercion bytecode, and collected in order. If more
//     than one value is returned, a stack marker is pushed first so the caller
//     can locate the boundary. Expressions are pushed in reverse order and
//     Return is emitted with the count.
//
//  3. Bare return (no expression): Return is emitted with a count of 0.
func (c *Compiler) compileReturn() error {
	// If there are deferred statements stored in the runtime
	// context, this will run them.
	c.b.Emit(bytecode.RunDefers)

	// Do we have named return values?
	if len(c.returnVariables) > 0 {
		c.b.Emit(bytecode.Push, bytecode.NewStackMarker(c.b.Name(), len(c.returnVariables)))

		// If so, we need to push the return values on the stack
		// in the reverse order they were declared.
		for i := len(c.returnVariables) - 1; i >= 0; i = i - 1 {
			if err := c.ReferenceSymbol(c.returnVariables[i].Name); err != nil {
				return err
			}

			c.b.Emit(bytecode.Load, c.returnVariables[i].Name)
		}

		c.b.Emit(bytecode.Return, len(c.returnVariables))

		// If there are no return expressions, we're done.
		if !c.isStatementEnd() {
			return c.compileError(errors.ErrInvalidReturnValues)
		}

		return nil
	}

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
