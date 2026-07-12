package compiler

import (
	"fmt"

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
	//     by DropToMarker.  We emit Load + Push 1 + Add/Sub + Store +
	//     DropToMarker directly into c.b (see BUG-63 in docs/ISSUES.md for
	//     why the DropToMarker step matters).
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
			// Emit: Load x, Push 1, Add/Sub, Store x, DropToMarker
			//
			// Fix BUG-63: before this fix, this branch ended right after
			// "Store x" and returned immediately -- it never emitted a
			// DropToMarker. Here is why that mattered:
			//
			// A few lines above (inside c.assignmentTarget(), called at the
			// very top of compileAssignment), the compiler already emitted
			// "Push StackMarker(let)" into c.b. Think of a StackMarker as a
			// sentinel value pushed onto the runtime value stack purely so
			// that later code can find its way back to "the stack depth we
			// started at" -- like a bookmark. Every other path through this
			// function honors the contract that whoever placed that bookmark
			// is responsible for removing it again with a matching
			// "DropToMarker" instruction (which pops values off the stack
			// until it finds and removes the bookmark). The qualified-lvalue
			// branch just below this one does that correctly, because it
			// appends the full storeLValue bytecode fragment, which always
			// ends in DropToMarker.
			//
			// This branch, however, built its own hand-rolled Load/Push/
			// Add-or-Sub/Store sequence instead of using storeLValue, and
			// returned without ever emitting the matching DropToMarker. The
			// bookmark was left on the stack permanently. A statement like
			// "x++" is supposed to leave the stack exactly as it found it --
			// the same way "x = x + 1" does -- but instead it grew the stack
			// by one leaked marker every time it ran. A later function call
			// in the same scope (or an enclosing one) would then see that
			// stray marker mixed in with its own arguments or return values,
			// typically surfacing as "function did not return the expected
			// number of values".
			//
			// The fix is the DropToMarker call below: it removes the exact
			// "let" marker that assignmentTarget() pushed, restoring the
			// stack to the depth it had before this statement began.
			t := data.String(firstInstr.Operand)

			if err = c.ReferenceSymbol(t); err != nil {
				return err
			}

			c.b.Emit(bytecode.Load, t)
			c.b.Emit(bytecode.Push, 1)
			c.b.Emit(autoMode)
			c.b.Emit(bytecode.Store, t)
			c.b.Emit(bytecode.DropToMarker, bytecode.NewStackMarker("let"))

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

	// Detect whether the right-hand side begins with a channel-receive
	// operator (<-), but ONLY for the two-value form (v, ok := <-ch;
	// multipleTargets is true). That form's storeLValue was built expecting
	// the [StackMarker("receive"), ok, datum] shape the ReceiveChannel
	// opcode produces (see below) -- and Go's own grammar requires a
	// two-value receive's right-hand side to be bare "<-ch" with nothing
	// else, so greedily treating "the rest of the RHS" as the channel
	// expression is always correct here.
	//
	// The single-value form (x := <-ch) is deliberately NOT special-cased
	// here anymore (BUG-62/BUG-72, July 2026): "<-" is left untouched and
	// handled by the ordinary expression parser below, which recognizes it
	// as a general expression atom (expressionAtom in expr_atom.go) and
	// already leaves a plain received value on the stack. That's what
	// makes "x := <-ch + 1" work -- treating the whole remainder as one
	// channel expression here would have misparsed it as "<-(ch + 1)"
	// instead of "(<-ch) + 1". storeLValue no longer bakes in a channel
	// receive for this form either (see the isChan comment in
	// lvalue.go's assignmentTarget), so an ordinary Store is all that's
	// needed regardless of whether "<-" appears in the expression at all.
	isChannelReceive := false

	if c.flags.multipleTargets {
		isChannelReceive = c.t.IsNext(tokenizer.ChannelReceiveToken)
	}

	// Seems like a simple assignment at this point, so parse the expression
	// to be assigned, emit the code for that expression, and then emit the code
	// that will store the result in the lvalue.
	expressionCode, err := c.Expression(true)
	if err != nil {
		return err
	}

	// Is this a comma-separated list of RHS expressions, as in
	// "a, b, c = 10, 20, 30"? A single expression that itself yields
	// multiple values (a multi-return call, or a channel receive already
	// handled above) is not followed by a comma, so this only triggers
	// for a genuine parallel-assignment expression list.
	if !isChannelReceive && c.t.IsNext(tokenizer.CommaToken) {
		return c.compileParallelAssignment(expressionCode, storeLValue)
	}

	c.b.Append(expressionCode)

	// Two-value channel receive: v, ok := <-ch
	//
	// When both conditions hold:
	//   - isChannelReceive is true  (we saw and consumed "<-")
	//   - multipleTargets is true   (the LHS was a comma-separated list)
	//
	// …the storeLValue produced by assignmentTargetList starts with
	// StackCheck 2, which requires exactly two values above a stack marker.
	// The single Load "ch" instruction emitted above pushes only the channel
	// object (one value); that fails the StackCheck.
	//
	// ReceiveChannel fixes this: it pops the channel, performs the receive,
	// and pushes [StackMarker("receive"), ok, datum] so that StackCheck 2
	// sees two items (datum + ok) above the marker.  The storeLValue that
	// follows then stores datum → v and ok → ok in the correct order.
	//
	// Single-value receive (v := <-ch) does NOT reach this branch because
	// isChannelReceive is never set to true unless multipleTargets already
	// is (see above) -- it is compiled as an ordinary expression instead,
	// via expressionAtom's ChannelReceiveToken case, which leaves a plain
	// received value on the stack for the ordinary Store below to consume.
	if isChannelReceive && c.flags.multipleTargets {
		c.b.Emit(bytecode.ReceiveChannel)
		c.b.Append(storeLValue)

		return nil
	}

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

// compileParallelAssignment compiles a Go-style parallel assignment whose
// right-hand side is a comma-separated list of expressions, one per target,
// as in:
//
//	a, b, c = 10, 20, 30
//	x, y = y, x
//
// The caller has already compiled the left-hand side (storeLValue, which
// contains a StackCheck instruction followed by one Store per target) and the
// first RHS expression (firstExpr); the tokenizer has just consumed the comma
// that follows it. This function parses the remaining expressions, verifies
// the count matches the number of targets, and emits the values in reverse
// order behind a fresh stack marker -- the same convention compileReturn uses
// for multi-value returns -- so that the first target ends up on top of the
// stack for storeLValue's first Store instruction to consume.
func (c *Compiler) compileParallelAssignment(firstExpr *bytecode.ByteCode, storeLValue *bytecode.ByteCode) error {
	expressions := []*bytecode.ByteCode{firstExpr}

	for {
		expr, err := c.Expression(true)
		if err != nil {
			return err
		}

		expressions = append(expressions, expr)

		if !c.t.IsNext(tokenizer.CommaToken) {
			break
		}
	}

	if targetCount := storeLValue.StoreCount(); targetCount != len(expressions) {
		return c.compileError(errors.ErrAssignmentCount,
			fmt.Sprintf("%d variables but %d values", targetCount, len(expressions)))
	}

	c.b.Emit(bytecode.Push, bytecode.NewStackMarker("let", len(expressions)))

	for i := len(expressions) - 1; i >= 0; i = i - 1 {
		c.b.Append(expressions[i])
	}

	c.b.Append(storeLValue)

	return nil
}
