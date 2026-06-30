package compiler

import (
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// compileAssignment is used to compile assignment statements. Here's a
// step-by-step breakdown of what the function does:
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
//	If the assignment was an any unwrap operation, it modifies the
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

	if c.t.Peek(1).Is(tokenizer.IncrementToken) {
		autoMode = bytecode.Add
	}

	if c.t.Peek(1).Is(tokenizer.DecrementToken) {
		autoMode = bytecode.Sub
	}

	// If there was an auto increment/decrement, generate the appropriate
	// load-modify-store sequence. Two structural forms are supported:
	//
	//  1. Simple variable:   x++  or  x--
	//     The storeLValue has exactly two instructions: Store "x" followed
	//     by DropToMarker.  We emit Load + Push 1 + Add/Sub + Dup + Store
	//     directly into c.b, matching the existing behavior.
	//
	//  2. Qualified lvalue:  a[i]++  or  s.field++
	//     The storeLValue contains a load path (Load base, <index exprs>),
	//     a StoreIndex instruction, and DropToMarker.  We rebuild the load
	//     half with a LoadIndex at the end to read the current value, apply
	//     the arithmetic, and then append the full storeLValue (which runs
	//     StoreIndex + DropToMarker) to write the result back.
	if autoMode != bytecode.NoOperation {
		// Advance past the ++ or -- token so the caller's tokenizer position
		// is left after the operator.
		c.t.Advance(1)

		// Peek at the first instruction of the store lvalue to determine
		// whether this is a simple variable or a qualified lvalue (array
		// element or struct field).
		firstInstr := storeLValue.Instruction(0)

		// A simple variable produces exactly two instructions: Store "name"
		// and DropToMarker.  Any other shape is a qualified lvalue.
		isSimpleLValue := storeLValue.Mark() == 2 &&
			firstInstr != nil &&
			firstInstr.Operation == bytecode.Store

		if isSimpleLValue {
			// --- Simple variable: x++ or x-- ---
			//
			// Emit: Load x, Push 1, Add/Sub, Dup, Store x
			//
			// The Dup leaves a copy of the new value on the stack (below the
			// "let" marker that assignmentTarget pushed) so the result is
			// available to callers that treat ++ as an expression. The stack
			// is cleaned up by the next DropToMarker in the generated code.
			t := data.String(firstInstr.Operand)

			if err = c.ReferenceSymbol(t); err != nil {
				return err
			}

			c.b.Emit(bytecode.Load, t)
			c.b.Emit(bytecode.Push, 1)
			c.b.Emit(autoMode)
			c.b.Emit(bytecode.Dup)
			c.b.Emit(bytecode.Store, t)

			return nil
		}

		// --- Qualified lvalue: a[i]++ or s.field++ ---
		//
		// The storeLValue structure for a qualified lvalue is:
		//   [0]          Load "base"       — load the container (array or struct)
		//   [1..n-3]     <index exprs>     — push each intermediate index
		//   [n-2]        StoreIndex        — write value back (patched from LoadIndex)
		//   [n-1]        DropToMarker      — clean up the "let" stack marker
		//
		// We need the second-to-last instruction to be StoreIndex.  Any
		// other form (e.g. pointer dereference) is not supported for ++/--.
		storeInstrIdx := storeLValue.Mark() - 2
		storeInstr := storeLValue.Instruction(storeInstrIdx)

		if storeInstr == nil || storeInstr.Operation != bytecode.StoreIndex {
			// Not a form we know how to auto-increment (e.g. pointer deref).
			return c.compileError(errors.ErrInvalidAuto)
		}

		// Step 1 — Emit the "load" path.
		//
		// Copy every instruction from the storeLValue up to (but not
		// including) the StoreIndex.  These are the Load + index-push
		// instructions that navigate to the element.
		for i := 0; i < storeInstrIdx; i++ {
			instr := storeLValue.Instruction(i)
			c.b.Emit(instr.Operation, instr.Operand)
		}

		// Emit LoadIndex to read the current value of a[i] or s.field.
		// Pass the same operand that StoreIndex carries: nil means pop the
		// index from the stack (the preceding Push instruction did that);
		// a non-nil operand is a compile-time constant folded by the
		// optimizer and is used directly by the instruction handler.
		c.b.Emit(bytecode.LoadIndex, storeInstr.Operand)

		// Step 2 — Apply the increment or decrement.
		c.b.Emit(bytecode.Push, 1)
		c.b.Emit(autoMode)

		// Step 3 — Write the result back via the full storeLValue.
		//
		// storeLValue re-navigates to the element (Load base + index exprs)
		// and then writes the top-of-stack value with StoreIndex.  The
		// DropToMarker at its end cleans up the "let" marker that
		// assignmentTarget pushed into c.b before we were called.
		c.b.Append(storeLValue)

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
		return c.compileError(errors.ErrMissingAssignment)
	}

	// If we are at the end of the statement, then this is an error.
	if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
		return c.compileError(errors.ErrMissingExpression)
	}

	// Handle implicit operators, like += or /= which do both
	// a math operation and an assignment.
	mode := bytecode.NoOperation
	tok := c.t.Peek(0)

	switch {
	case tok.Is(tokenizer.AddAssignToken):
		mode = bytecode.Add

	case tok.Is(tokenizer.SubtractAssignToken):
		mode = bytecode.Sub

	case tok.Is(tokenizer.MultiplyAssignToken):
		mode = bytecode.Mul

	case tok.Is(tokenizer.DivideAssignToken):
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

	// If this assignment was an any unwrap operation, then
	// we need to modify the lvalue store by removing the last bytecode
	// (which is a DropToMarker). Then add code that checks to see if there
	// was abandoned info on the stack that should trigger an error when false.
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
