package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileAssignment compiles an assignment statement.
func (c *Compiler) compileAssignment() *errors.EgoError {
	storeLValue, err := c.assignmentTarget()
	if !errors.Nil(err) {
		return err
	}

	// Check for auto-increment or decrement
	autoMode := bytecode.Load

	if c.t.Peek(1) == "++" {
		autoMode = bytecode.Add
	}

	if c.t.Peek(1) == "--" {
		autoMode = bytecode.Sub
	}

	if autoMode != bytecode.Load {
		t := datatypes.GetString(storeLValue.GetInstruction(0).Operand)

		c.b.Emit(bytecode.Load, t)
		c.b.Emit(bytecode.Push, 1)
		c.b.Emit(autoMode)
		c.b.Emit(bytecode.Dup)
		c.b.Emit(bytecode.Store, t)
		c.t.Advance(1)

		return nil
	}

	// Not auto-anything, so verify that this is a legit assignment
	if !c.t.AnyNext(":=", "=", "<-") {
		return c.newError(errors.ErrMissingAssignment)
	}

	if c.t.AnyNext(";", tokenizer.EndOfTokens) {
		return c.newError(errors.ErrMissingExpression)
	}

	// If this is a construct like   x := <-ch   skip over the :=
	_ = c.t.IsNext("<-")

	expressionCode, err := c.Expression()
	if !errors.Nil(err) {
		return err
	}

	c.b.Append(expressionCode)
	c.b.Append(storeLValue)

	return nil
}
