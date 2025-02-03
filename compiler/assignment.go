package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileAssignment is used to compile assignment statements. Here's a step-by-step
// breakdown of what the function does:
//
//	It starts by marking the current position in the token stream.
//
//	It then generates the left-hand side (LHS) of the assignment statement.
//	This is the variable that will be assigned a value.
//
//	It checks if the next token is an increment or decrement operator. If
//	it is, it sets the autoMode variable to the corresponding bytecode
//	operation.
//
//	If an auto increment/decrement was found, it checks if the LHS is a
//	simple value by ensuring the LHS store code is only two instructions.
//	If it's not, it returns an error. If it is, it emits bytecode to load
//	the variable, increment/decrement it, duplicate the result, store it
//	back in the variable, and advance the token stream.
//
//	If there was no auto increment/decrement, it checks if the next token
//	is an assignment operator. If it's not, it returns an error.
//
//	If the token stream is at the end, it returns an error because an
//	expression is missing.
//
//	It then handles implicit operators like += or /= which do both a math
//	operation and an assignment. If an implicit operator is found, it backs
//	up the tokenizer to the LHS token, parses the expression, verifies the
//	next operator, parses the second term, emits the expressions and the
//	operator, and then stores the result using the LHS store code.
//
//	If it's a simple assignment, it parses the expression to be assigned,
//	emits the code for that expression, and then emits the code that will
//	store the result in the LHS.
//
//	If the assignment was an interface{} unwrap operation, it modifies the
//	LHS store code by removing the last bytecode, adds code that checks if
//	there was abandoned info on the stack that should trigger an error if
//	false, and then emits the LHS store code.
//
//	Finally, it appends the LHS store code to the bytecode and returns.
func (c *Compiler) compileAssignment() error {
	start := c.t.Mark()

	// Generate the lvalue code. This will be a series of instructions
	// used to store a result in the left side of the assignment statement.
	storeLValue, err := c.assignmentTarget()
	if err != nil {
		return err
	}

	// Set the flag indicating this is an active assignment statement. Make
	// sure it is turned off again when we're done.
	c.flags.inAssignment = true
	defer func() {
		c.flags.inAssignment = false
		c.flags.multipleTargets = false
	}()

	if storeLValue.StoreCount() > 1 {
		c.flags.multipleTargets = true
	}
	// Check for auto-increment or decrement
	autoMode := bytecode.NoOperation

	if c.t.Peek(1) == tokenizer.IncrementToken {
		autoMode = bytecode.Add
	}

	if c.t.Peek(1) == tokenizer.DecrementToken {
		autoMode = bytecode.Sub
	}

	// If there was an auto increment/decrement, make sure the LValue is
	// a simple value. We can check this easily by ensuring the LValue store
	// code is only two instructions (which will always be a "store" followed
	// by a "drop to marker").
	if autoMode != bytecode.NoOperation {
		if storeLValue.Mark() > 2 {
			return c.error(errors.ErrInvalidAuto)
		}

		t := data.String(storeLValue.Instruction(0).Operand)

		if err = c.ReferenceSymbol(t); err != nil {
			return err
		}

		c.b.Emit(bytecode.Load, t)
		c.b.Emit(bytecode.Push, 1)
		c.b.Emit(autoMode)
		c.b.Emit(bytecode.Dup)
		c.b.Emit(bytecode.Store, t)
		c.t.Advance(1)

		return nil
	}

	// Not auto-anything, so verify that this is a legit assignment by
	// verifying that the next token is an arithmetic or assignment operator.
	if !c.t.AnyNext(tokenizer.DefineToken,
		tokenizer.AssignToken,
		tokenizer.ChannelReceiveToken,
		tokenizer.AddAssignToken,
		tokenizer.SubtractAssignToken,
		tokenizer.MultiplyAssignToken,
		tokenizer.DivideAssignToken) {
		return c.error(errors.ErrMissingAssignment)
	}

	// If we are at the end of the statement, then this is an error.
	if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
		return c.error(errors.ErrMissingExpression)
	}

	// Handle implicit operators, like += or /= which do both
	// a math operation and an assignment.
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

	// If we found an explicit operation, then let's do it.
	if mode != bytecode.NoOperation {
		// Back the tokenizer up to the lvalue token, because we need to
		// re-parse this as an expression instead of an lvalue to get one
		// of the terms for the operation.
		c.t.Set(start)

		// Parse the expression.
		e1, err := c.Expression(true)
		if err != nil {
			return err
		}

		// Verify that the next operator is still valid after
		// processing the implicit operator.
		if !c.t.AnyNext(tokenizer.AddAssignToken,
			tokenizer.SubtractAssignToken,
			tokenizer.MultiplyAssignToken,
			tokenizer.DivideAssignToken) {
			return errors.ErrMissingAssignment
		}

		// And then parse the second term that follows the implicit operator.
		e2, err := c.Expression(true)
		if err != nil {
			return err
		}

		// Emit the expressions and the operator, and then store using the code
		// generated for the lvalue.
		c.b.Append(e1)
		c.b.Append(e2)
		c.b.Emit(mode)
		c.b.Append(storeLValue)

		return nil
	}

	// If this is a construct like   x := <-ch   skip over the :=
	_ = c.t.IsNext(tokenizer.ChannelReceiveToken)

	// Seems like a simple assignment at this point, so parse the expression
	// to be assigned, emit the code for that expression, and then emit the code
	// that will store the result in the lvalue.
	expressionCode, err := c.Expression(true)
	if err != nil {
		return err
	}

	c.b.Append(expressionCode)

	// If this assignment was an interfaace{} unwrap operation, then
	// we need to modify the lvalue store by removing the last bytecode
	// (which is a DropToMarker). Then add code that checks to see if there
	// was abandoned info on the stack that should trigger an error if false.
	if c.flags.hasUnwrap {
		if storeLValue.StoreCount() < 2 {
			storeLValue.Remove(storeLValue.Mark() - 1)
			c.b.Emit(bytecode.Swap)
			c.b.Append(storeLValue)
			c.b.Emit(bytecode.IfError, errors.ErrTypeMismatch)
			c.b.Emit(bytecode.DropToMarker, bytecode.NewStackMarker("let"))

			return nil
		} else {
			c.b.Emit(bytecode.Swap)
		}
	}

	c.b.Append(storeLValue)

	return nil
}
