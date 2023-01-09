package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileAssignment compiles an assignment statement.
func (c *Compiler) compileAssignment() error {
	start := c.t.Mark()

	storeLValue, err := c.assignmentTarget()
	if err != nil {
		return err
	}

	// Check for auto-increment or decrement
	autoMode := bytecode.NoOperation

	if c.t.Peek(1) == tokenizer.IncrementToken {
		autoMode = bytecode.Add
	}

	if c.t.Peek(1) == tokenizer.DecrementToken {
		autoMode = bytecode.Sub
	}

	if autoMode != bytecode.NoOperation {
		t := datatypes.String(storeLValue.Instruction(0).Operand)

		c.b.Emit(bytecode.Load, t)
		c.b.Emit(bytecode.Push, 1)
		c.b.Emit(autoMode)
		c.b.Emit(bytecode.Dup)
		c.b.Emit(bytecode.Store, t)
		c.t.Advance(1)

		return nil
	}

	// Not auto-anything, so verify that this is a legit assignment
	if !c.t.AnyNext(tokenizer.DefineToken,
		tokenizer.AssignToken,
		tokenizer.ChannelReceiveToken,
		tokenizer.AddAssignToken,
		tokenizer.SubtractAssignToken,
		tokenizer.MultiplyAssignToken,
		tokenizer.DivideAssignToken) {
		return c.newError(errors.ErrMissingAssignment)
	}

	if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
		return c.newError(errors.ErrMissingExpression)
	}

	// Handle implicit operators
	mode := bytecode.NoOperation

	switch c.t.Peek(0) {
	case tokenizer.AddAssignToken:
		mode = bytecode.Add

	case tokenizer.SubtractAssignToken:
		mode = bytecode.Sub

	case tokenizer.MultiplyAssignToken:
		mode = bytecode.Mul

	case tokenizer.DivideAssignToken:
		mode = bytecode.Div
	}

	if mode != bytecode.NoOperation {
		c.t.Set(start)

		e1, err := c.Expression()
		if err != nil {
			return err
		}

		if !c.t.AnyNext(tokenizer.AddAssignToken,
			tokenizer.SubtractAssignToken,
			tokenizer.MultiplyAssignToken,
			tokenizer.DivideAssignToken) {
			return errors.EgoError(errors.ErrMissingAssignment)
		}

		e2, err := c.Expression()
		if err != nil {
			return err
		}

		c.b.Append(e1)
		c.b.Append(e2)
		c.b.Emit(mode)
		c.b.Append(storeLValue)

		return nil
	}

	// If this is a construct like   x := <-ch   skip over the :=
	_ = c.t.IsNext(tokenizer.ChannelReceiveToken)

	expressionCode, err := c.Expression()
	if err != nil {
		return err
	}

	c.b.Append(expressionCode)
	c.b.Append(storeLValue)

	return nil
}
