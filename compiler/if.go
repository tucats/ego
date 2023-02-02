package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileIf compiles conditional statments. The verb is already
// removed from the token stream.
func (c *Compiler) compileIf() error {
	if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
		return c.error(errors.ErrMissingExpression)
	}

	conditionalAssignent := false

	// Is there an assignment statement prefix before the conditional?
	if c.isAssignmentTarget() {
		c.b.Emit(bytecode.PushScope)

		if err := c.compileAssignment(); err != nil {
			return err
		}

		// Must be followed by a semicolon for the actual conditional
		if !c.t.IsNext(tokenizer.SemicolonToken) {
			return c.error(errors.ErrMissingSemicolon)
		}

		conditionalAssignent = true
	}

	// Compile the conditional expression
	bc, err := c.Expression()
	if err != nil {
		return err
	}

	c.b.Append(bc)

	b1 := c.b.Mark()

	c.b.Emit(bytecode.BranchFalse, 0)

	// Compile the statement to be executed if true
	err = c.compileRequiredBlock()
	if err != nil {
		return err
	}

	// If there's an else clause, branch around it.
	if c.t.IsNext(tokenizer.ElseToken) {
		b2 := c.b.Mark()

		c.b.Emit(bytecode.Branch, 0)
		_ = c.b.SetAddressHere(b1)

		err = c.compileRequiredBlock()
		if err != nil {
			return err
		}

		_ = c.b.SetAddressHere(b2)
	} else {
		_ = c.b.SetAddressHere(b1)
	}

	// If we had an assignment as part of the condition, discard
	// the scope in which it was created.
	if conditionalAssignent {
		c.b.Emit(bytecode.PopScope)
	}

	return nil
}
