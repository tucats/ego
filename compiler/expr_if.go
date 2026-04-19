package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// Compile the "if" expression atom. This is patterned on the if statement.
//
//	x := if x < 5 { "true" } else { "false" }
//
// IF the conditional expression x<5 is true, the expression result is "true",
// else it is "false".  The entire item from the "if" to the trailing "}" is
// a single expression atom, and can be used with more complex expressions.
func (c *Compiler) ifExpression() error {
	// The "if" token itself has already been removed.
	bc, err := c.Expression(true)
	if err != nil {
		return err
	}

	// Emit the expression, which must generate a conditional value
	// on the stack. Then, emit code that tests if the conditional value
	// is equal to false, which would result in jumping to the else block
	c.b.Append(bc)

	falseMarker := c.b.Mark()

	c.b.Emit(bytecode.BranchFalse, 0)

	// The expressions must be inside braces for syntactic harmony with if statements
	if !c.t.IsNext(tokenizer.BlockBeginToken) {
		return c.compileError(errors.ErrMissingBlock)
	}

	// Now parse and emit a value expression for the "true" case.
	bc, err = c.Expression(true)
	if err != nil {
		return err
	}

	// Must be followed by an end-of-block
	if !c.t.IsNext(tokenizer.BlockEndToken) {
		return c.compileError(errors.ErrMissingEndOfBlock)
	}

	// Emit the expression value, and a branch to the
	// exit location.
	c.b.Append(bc)

	exitMarker := c.b.Mark()

	c.b.Emit(bytecode.Branch, 0)

	// Patch up the false jump to here, where we will
	// process an else {value}
	c.b.SetAddressHere(falseMarker)

	// Now the else block
	if !c.t.IsNext(tokenizer.ElseToken) {
		return c.compileError(errors.ErrMissingElse)
	}

	if !c.t.IsNext(tokenizer.BlockBeginToken) {
		return c.compileError(errors.ErrMissingBlock)
	}

	// Now parse and emit a value expression for the "false" case.
	bc, err = c.Expression(true)
	if err != nil {
		return err
	}

	// Must be followed by an end-of-block
	if !c.t.IsNext(tokenizer.BlockEndToken) {
		return c.compileError(errors.ErrMissingEndOfBlock)
	}

	// Emit the expression value, and update the branch to the exit location.
	c.b.Append(bc)

	c.b.SetAddressHere(exitMarker)

	return nil
}
