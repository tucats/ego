package compiler

import (
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/tokenizer"
)

// For compiles the loop statement. This has four syntax types that
// can be specified.
// 1. There are three clauses which are separated by ";", followed
//    by a statement or block that is run as described by the loop
//    index variable conditions.
//
// 2. There can be a range operation which creates an implied loop
//    using each member of the array or struct.
//
// 3. There can be a simple conditional expression. The loop runs
//    until the condition expression is false. The condition is
//    tested at the start of every loop, so a condition that is
//    initially false never runs the loop
//
// 4. A for{} with no condition, loop, or range expression. This
//    form _requires_ that there be at least one break statement
//    inside the loop, which algorithmically stops the loop
func (c *Compiler) For() error {
	c.b.Emit(bytecode.PushScope)
	// Is this a for{} with no conditional or iterator?
	if c.t.Peek(1) == "{" {
		// Make a new scope and emit the test expression.
		c.PushLoop(forLoopType)

		// Remember top of loop. Three is no looping or condition code associated
		// with the top of the loop.
		b1 := c.b.Mark()

		// Compile loop body
		err := c.Statement()
		if err != nil {
			return err
		}

		// Branch back to start of loop
		c.b.Emit(bytecode.Branch, b1)

		for _, fixAddr := range c.loops.continues {
			_ = c.b.SetAddress(fixAddr, b1)
		}

		// Update any break statements. If there are no breaks, this is an illegal loop construct
		if len(c.loops.breaks) == 0 {
			return c.NewError(LoopExitError)
		}

		for _, fixAddr := range c.loops.breaks {
			_ = c.b.SetAddressHere(fixAddr)
		}
		c.PopLoop()

		return err
	}

	// Is this the two-value range thing?
	indexName := c.t.Peek(1)
	valueName := ""
	if tokenizer.IsSymbol(indexName) && (c.t.Peek(2) == ",") {
		c.t.Advance(2)
		valueName = c.t.Peek(1)
	}
	indexName = c.Normalize(indexName)
	valueName = c.Normalize(valueName)

	// if not an lvalue, assume conditional mode
	if !c.IsLValue() {
		bc, err := c.Expression()
		if err != nil {
			return c.NewError(MissingForLoopInitializerError)
		}

		// Make a point of seeing if this is a constant value, which
		// will require a break statement. We check to see if the test
		// loads any symbols or calls any functions.
		ops := bc.Opcodes()
		isConstant := true

		for _, b := range ops {
			if b.Operation == bytecode.Load ||
				b.Operation == bytecode.LoadIndex ||
				b.Operation == bytecode.Call ||
				b.Operation == bytecode.LocalCall ||
				b.Operation == bytecode.Member ||
				b.Operation == bytecode.ClassMember {
				isConstant = false

				break
			}
		}
		// Make a new scope and emit the test expression.
		c.PushLoop(conditionalLoopType)
		// Remember top of loop and generate test
		b1 := c.b.Mark()
		c.b.Append(bc)
		b2 := c.b.Mark()
		c.b.Emit(bytecode.BranchFalse, 0)

		// Compile loop body
		opcount := c.b.Mark()
		stmts := c.statementCount

		err = c.Statement()
		if err != nil {
			return err
		}
		// If we didn't emit anything other than
		// the AtLine then this is an invalid loop
		if c.b.Mark() <= opcount+1 {
			return c.NewError(LoopBodyError)
		}

		// Uglier test, but also needs doing. If there was a statement, but
		// it was a block that did not contain any statments, also empty body.
		wasBlock := c.b.Opcodes()[len(c.b.Opcodes())-1]
		if wasBlock.Operation == bytecode.PopScope && stmts == c.statementCount-1 {
			return c.NewError(LoopBodyError)
		}
		// Branch back to start of loop
		c.b.Emit(bytecode.Branch, b1)
		for _, fixAddr := range c.loops.continues {
			_ = c.b.SetAddress(fixAddr, b1)
		}

		// Update the loop exit instruction, and any breaks
		_ = c.b.SetAddressHere(b2)

		if isConstant && len(c.loops.breaks) == 0 {
			return c.NewError(LoopExitError)
		}
		for _, fixAddr := range c.loops.breaks {
			_ = c.b.SetAddressHere(fixAddr)
		}
		c.b.Emit(bytecode.PopScope)
		c.PopLoop()

		return nil
	}

	indexStore, err := c.LValue()
	if err != nil {
		return err
	}
	if !c.t.IsNext(":=") {
		return c.NewError(MissingLoopAssignmentError)
	}

	// Do we compile a range?
	if c.t.IsNext("range") {
		c.PushLoop(rangeLoopType)

		// For a range, the index and value targets must be simple names, and cannot
		// be real lvalues. The actual thing we range is on the stack.
		bc, err := c.Expression()
		if err != nil {
			return c.NewError(err.Error())
		}
		c.b.Append(bc)
		c.b.Emit(bytecode.RangeInit, []interface{}{indexName, valueName})

		// Remember top of loop
		b1 := c.b.Mark()

		// Get new index and value. Destination is as-yet unknown.
		c.b.Emit(bytecode.RangeNext, 0)

		// Loop body
		err = c.Statement()
		if err != nil {
			return err
		}

		// Make note of the loop end point where continues fall.
		b3 := c.b.Mark()
		// Branch back to start of loop
		c.b.Emit(bytecode.Branch, b1)
		for _, fixAddr := range c.loops.continues {
			_ = c.b.SetAddress(fixAddr, b3)
		}

		_ = c.b.SetAddressHere(b1)

		for _, fixAddr := range c.loops.breaks {
			_ = c.b.SetAddressHere(fixAddr)
		}
		c.PopLoop()
		if indexName != "" && indexName != "_" {
			c.b.Emit(bytecode.SymbolDelete, indexName)
		}
		if valueName != "" && valueName != "_" {
			c.b.Emit(bytecode.SymbolDelete, valueName)
		}
		c.b.Emit(bytecode.PopScope)

		return nil
	}

	// Nope, normal numeric loop conditions. At this point there should not
	// be an index variable defined.
	if indexName == "" && valueName != "" {
		return c.NewError(InvalidLoopIndexError)
	}
	c.PushLoop(indexLoopType)

	// The expression is the initial value of the loop.
	initializerCode, err := c.Expression()
	if err != nil {
		return err
	}
	c.b.Append(initializerCode)
	c.b.Append(indexStore)
	if !c.t.IsNext(";") {
		return c.NewError(MissingSemicolonError)
	}

	// Now get the condition clause that tells us if the loop
	// is still executing.
	condition, err := c.Expression()
	if err != nil {
		return err
	}

	if !c.t.IsNext(";") {
		return c.NewError(MissingSemicolonError)
	}

	// Finally, get the clause that updates something
	// (nominally the index) to eventually trigger the
	// loop condition.
	incrementStore, err := c.LValue()
	if err != nil {
		return err
	}

	if !c.t.IsNext("=") {
		return c.NewError(MissingEqualError)
	}
	incrementCode, err := c.Expression()
	if err != nil {
		return err
	}

	// Top of loop body starts here
	b1 := c.b.Mark()

	// Emit the test condition
	c.b.Append(condition)
	b2 := c.b.Mark()
	c.b.Emit(bytecode.BranchFalse, 0)

	// Loop body goes next
	err = c.Statement()
	if err != nil {
		return err
	}

	// Emit increment code, and loop. Finally, mark the exit location from
	// the condition test for the loop.
	c.b.Append(incrementCode)
	c.b.Append(incrementStore)
	c.b.Emit(bytecode.Branch, b1)
	_ = c.b.SetAddressHere(b2)

	for _, fixAddr := range c.loops.continues {
		_ = c.b.SetAddress(fixAddr, b1)
	}

	for _, fixAddr := range c.loops.breaks {
		_ = c.b.SetAddressHere(fixAddr)
	}
	c.b.Emit(bytecode.PopScope)
	c.PopLoop()

	return nil
}

// Break compiles a break statement. This is a branch, and the
// destination is fixed up when the loop compilation finishes.
// As such, the address of the fixup is added to the breaks list
// in the compiler context.
func (c *Compiler) Break() error {
	if c.loops == nil {
		return c.NewError(InvalidLoopControlError)
	}
	fixAddr := c.b.Mark()
	c.b.Emit(bytecode.Branch, 0)
	c.loops.breaks = append(c.loops.breaks, fixAddr)

	return nil
}

// Continue compiles a continue statement. This is a branch, and the
// destination is fixed up when the loop compilation finishes.
// As such, the address of the fixup is added to the continues list
// in the compiler context.
func (c *Compiler) Continue() error {
	if c.loops == nil {
		return c.NewError(InvalidLoopControlError)
	}
	fixAddr := c.b.Mark()
	c.b.Emit(bytecode.Branch, 0)
	c.loops.continues = append(c.loops.continues, fixAddr)

	return nil
}

// PushLoop creates a new loop context and adds it to the top of the
// loop stack. This stack retains information about the loop type and
// the accumulation of breaks and continues that are specfied within
// this loop body.  A break or continue _only_ applies to the loop scope
// in which it occurs.
func (c *Compiler) PushLoop(loopType int) {
	loop := Loop{
		Type:      loopType,
		breaks:    make([]int, 0),
		continues: make([]int, 0),
		Parent:    c.loops,
	}
	c.loops = &loop
}

// PopLoop discards the top-most loop context on the loop stack.
func (c *Compiler) PopLoop() {
	if c.loops != nil {
		c.loops = c.loops.Parent
	} else {
		ui.Debug(ui.ByteCodeLogger, "=== loop stack empty")
	}
}
