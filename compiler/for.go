package compiler

import (
	"fmt"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// Define the types of for loops we can compile. This determines how to
// handle increment and end-of-range tests.
type runtimeLoopType int

const (
	indexLoopType       runtimeLoopType = 1
	rangeLoopType       runtimeLoopType = 2
	forLoopType         runtimeLoopType = 3
	conditionalLoopType runtimeLoopType = 4
)

// These are used to generate index names when needed for range loops when the "_"
// variable is used.
var indexNameLock = &sync.Mutex{}
var indexNameCounter int

// compileFor compiles the loop statement. This has four syntax types that
// can be specified.
//
//  1. There are three clauses which are separated by tokenizer.SemicolonToken, followed
//     by a statement or block that is run as described by the loop
//     index variable conditions.
//
//  2. There can be a range operation which creates an implied loop
//     using each member of the array or struct.
//
//  3. There can be a simple conditional expression. The loop runs
//     until the condition expression is false. The condition is
//     tested at the start of every loop, so a condition that is
//     initially false never runs the loop
//
//  4. A for{} with no condition, loop, or range expression.
//     this form _requires_ that there be at least one break
//     statement inside the loop, which algorithmically stops
//     the loop
func (c *Compiler) compileFor() error {
	if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
		return c.compileError(errors.ErrMissingExpression)
	}

	if c.t.IsNext(tokenizer.EmptyBlockToken) {
		return c.compileError(errors.ErrLoopExit)
	}

	c.b.Emit(bytecode.PushScope, bytecode.ForScope)

	// Is this a for{} with no conditional or iterator?
	if c.t.Peek(1).Is(tokenizer.BlockBeginToken) {
		return c.simpleFor()
	}

	// Is this the two-value range thing?
	indexName := c.t.Peek(1)
	valueName := tokenizer.EmptyToken

	if indexName.IsIdentifier() && c.t.Peek(2).Is(tokenizer.CommaToken) {
		c.t.Advance(2)
		valueName = c.t.Peek(1)
	}

	indexNameSpelling := c.normalize(indexName.Spelling())
	valueNameSpelling := c.normalize(valueName.Spelling())

	// if not an lvalue, assume conditional mode
	if !c.isAssignmentTarget() {
		return c.conditionalFor()
	}

	indexStore, err := c.assignmentTarget()
	if err != nil {
		return err
	}

	// Because we put a marker on the stack during the
	// assignment, whenever we're done with the loop,
	// drop the marker.
	defer c.b.Emit(bytecode.DropToMarker)

	if !c.t.IsNext(tokenizer.DefineToken) {
		return c.compileError(errors.ErrMissingLoopAssignment)
	}

	// Do we compile a range?
	if c.t.IsNext(tokenizer.RangeToken) {
		return c.rangeFor(indexNameSpelling, valueNameSpelling)
	}

	// Nope, must be an old-school iteration form.
	return c.iterationFor(indexNameSpelling, valueNameSpelling, indexStore)
}

// loopStackPush creates a new loop context and adds it to the top of the
// loop stack. This stack retains information about the loop type and
// the accumulation of breaks and continues that are specified within
// this loop body.  A break or continue _only_ applies to the loop scope
// in which it occurs.
func (c *Compiler) loopStackPush(loopType runtimeLoopType) {
	c.loops = &loop{
		loopType:  loopType,
		breaks:    make([]int, 0),
		continues: make([]int, 0),
		parent:    c.loops,
	}
}

// loopStackPop discards the top-most loop context on the loop stack.
func (c *Compiler) loopStackPop() {
	if c.loops != nil {
		c.loops = c.loops.parent
	} else {
		ui.Log(ui.TraceLogger, "trace.loop.empty", nil)
	}
}

// Compile a simple for{} loop with no conditional or range. The
// loop body must contain a break statement or an error is reported.
func (c *Compiler) simpleFor() error {
	// Make a new scope and emit the test expression.
	c.loopStackPush(forLoopType)

	// Remember top of loop. There is no looping or condition code associated
	// with the top of the loop.
	b1 := c.b.Mark()

	// Compile loop body
	if err := c.compileRequiredBlock(); err != nil {
		return err
	}

	// Branch back to start of loop
	c.b.Emit(bytecode.Branch, b1)

	for _, fixAddr := range c.loops.continues {
		_ = c.b.SetAddress(fixAddr, b1)
	}

	// Update any break statements. If there are no breaks, this is an illegal loop construct
	if len(c.loops.breaks) == 0 {
		return c.compileError(errors.ErrLoopExit)
	}

	for _, fixAddr := range c.loops.breaks {
		_ = c.b.SetAddressHere(fixAddr)
	}

	c.loopStackPop()

	return nil
}

// Compile a conditional for-loop that runs as long as the condition
// is true.
func (c *Compiler) conditionalFor() error {
	bc, err := c.Expression(true)
	if err != nil {
		return c.compileError(errors.ErrMissingForLoopInitializer)
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
			b.Operation == bytecode.Member {
			isConstant = false

			break
		}
	}

	// Make a new scope and emit the test expression.
	c.loopStackPush(conditionalLoopType)

	// Remember top of loop and generate test
	b1 := c.b.Mark()

	c.b.Append(bc)

	b2 := c.b.Mark()

	c.b.Emit(bytecode.BranchFalse, 0)

	// Compile loop body
	opCount := c.b.Mark()
	statementCount := c.statementCount

	if err = c.compileRequiredBlock(); err != nil {
		return err
	}

	// If we didn't emit anything other than
	// the AtLine then this is an invalid loop
	if c.b.Mark() <= opCount+1 {
		return c.compileError(errors.ErrLoopBody)
	}

	// Uglier test, but also needs doing. If there was a statement, but
	// it was a block that did not contain any statements, also empty body.
	wasBlock := c.b.Opcodes()[len(c.b.Opcodes())-1]
	if wasBlock.Operation == bytecode.PopScope && statementCount == c.statementCount {
		return c.compileError(errors.ErrLoopBody)
	}

	// Branch back to start of loop
	c.b.Emit(bytecode.Branch, b1)

	for _, fixAddr := range c.loops.continues {
		_ = c.b.SetAddress(fixAddr, b1)
	}

	// Update the loop exit instruction, and any breaks
	_ = c.b.SetAddressHere(b2)

	if isConstant && len(c.loops.breaks) == 0 {
		return c.compileError(errors.ErrLoopExit)
	}

	for _, fixAddr := range c.loops.breaks {
		_ = c.b.SetAddressHere(fixAddr)
	}

	c.b.Emit(bytecode.PopScope)
	c.loopStackPop()

	return nil
}

// Compile a for-loop that is expressed by a range. The index variable name
// and value variable names are provided. If not specified by the user, they
// are empty strings.
func (c *Compiler) rangeFor(indexName, valueName string) error {
	c.loopStackPush(rangeLoopType)

	// For a range, the index and value targets must be simple names, and cannot
	// be real lvalues. The actual thing we range is on the stack.
	if err := c.emitExpression(); err != nil {
		return err
	}

	if indexName != defs.DiscardedVariable {
		c.DefineSymbol(indexName)
	} else {
		indexName = generateIndexName("idx")
	}

	if valueName != defs.DiscardedVariable {
		c.DefineSymbol(valueName)
	} else {
		valueName = generateIndexName("val")
	}

	c.b.Emit(bytecode.RangeInit, indexName, valueName)

	// Remember top of loop
	b1 := c.b.Mark()

	// Get new index and value. Destination is as-yet unknown.
	c.b.Emit(bytecode.RangeNext, 0)

	// Loop body
	if err := c.compileRequiredBlock(); err != nil {
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

	c.loopStackPop()
	c.b.Emit(bytecode.PopScope)

	return nil
}

// Compile a for loop using iterations with initializer, conditional, and
// iterator expressions before the function body.
func (c *Compiler) iterationFor(indexName, valueName string, indexStore *bytecode.ByteCode) error {
	// Normal numeric loop conditions. At this point there should not be an index
	// variable defined.
	if indexName == tokenizer.EmptyToken.Spelling() && valueName != tokenizer.EmptyToken.Spelling() {
		return c.compileError(errors.ErrInvalidLoopIndex)
	}

	c.loopStackPush(indexLoopType)

	// The expression is the initial value of the loop.
	initializerCode, err := c.Expression(true)
	if err != nil {
		return err
	}

	c.b.Append(initializerCode)
	c.b.Append(indexStore)

	if !c.t.IsNext(tokenizer.SemicolonToken) {
		return c.compileError(errors.ErrMissingSemicolon)
	}

	// Now get the condition clause that tells us if the loop
	// is still executing.
	condition, err := c.Expression(true)
	if err != nil {
		return err
	}

	if !c.t.IsNext(tokenizer.SemicolonToken) {
		return c.compileError(errors.ErrMissingSemicolon)
	}

	// Finally, get the clause that updates something
	// (nominally the index) to eventually trigger the
	// loop condition.
	incrementStore, err := c.assignmentTarget()
	if err != nil {
		return err
	}

	// Check for increment or decrement operators
	autoMode := bytecode.Load

	if c.t.Peek(1).Is(tokenizer.IncrementToken) {
		autoMode = bytecode.Add
	}

	if c.t.Peek(1).Is(tokenizer.DecrementToken) {
		autoMode = bytecode.Add
	}

	var incrementCode *bytecode.ByteCode

	// If increment mode was used, then the increment is just to add (or subtract)
	// 1 from the value.
	if autoMode != bytecode.Load {
		t := data.String(incrementStore.Instruction(0).Operand)
		incrementCode = bytecode.New("auto")

		if err := c.ReferenceSymbol(t); err != nil {
			return err
		}

		incrementCode.Emit(bytecode.Load, t)
		incrementCode.Emit(bytecode.Push, 1)
		incrementCode.Emit(autoMode)
		c.t.Advance(1)
	} else {
		// Not auto-increment/decrement, so must be a legit assignment and expression
		if !c.t.IsNext(tokenizer.AssignToken) {
			return c.compileError(errors.ErrMissingEqual)
		}

		incrementCode, err = c.Expression(true)
		if err != nil {
			return err
		}
	}

	// Top of loop body starts here
	b1 := c.b.Mark()

	// Emit the test condition
	c.b.Append(condition)

	b2 := c.b.Mark()

	c.b.Emit(bytecode.BranchFalse, 0)

	// Loop body goes next
	if err = c.compileRequiredBlock(); err != nil {
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
	c.loopStackPop()

	return nil
}

// compileBreak compiles a break statement. This is a branch, and the
// destination is fixed up when the loop compilation finishes.
// As such, the address of the fixup is added to the breaks list
// in the compiler context.
func (c *Compiler) compileBreak() error {
	if c.loops == nil {
		return c.compileError(errors.ErrInvalidLoopControl)
	}

	fixAddr := c.b.Mark()

	c.b.Emit(bytecode.Branch, 0)
	c.loops.breaks = append(c.loops.breaks, fixAddr)

	return nil
}

// compileContinue compiles a continue statement. This is a branch, and the
// destination is fixed up when the loop compilation finishes.
// As such, the address of the fixup is added to the continues list
// in the compiler context.
func (c *Compiler) compileContinue() error {
	if c.loops == nil {
		return c.compileError(errors.ErrInvalidLoopControl)
	}

	c.loops.continues = append(c.loops.continues, c.b.Mark())

	c.b.Emit(bytecode.Branch, 0)

	return nil
}

// Generate a unique synthetic variable name, using a string value
// as a component of the name. This is used to convert a range variable
// of "_" to a unique hidden name, for example.
func generateIndexName(descriptor string) string {
	indexNameLock.Lock()

	indexName := fmt.Sprintf("$%s%d", descriptor, indexNameCounter)
	indexNameCounter++

	indexNameLock.Unlock()

	return indexName
}
