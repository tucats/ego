package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// conditional compiles the ternary "?:" operator, which is a language extension
// available only when extensions are enabled. The syntax is:
//
//	condition ? valueIfTrue : valueIfFalse
//
// It is implemented with conditional branch instructions that skip over one of
// the two value expressions depending on the boolean result of the condition.
// If no "?" follows the condition expression, the call is a no-op (just the
// result of logicalOr is left on the stack).
func (c *Compiler) conditional() error {
	if err := c.logicalOr(); err != nil {
		return err
	}

	// Ternary operator is only recognized when extensions are enabled.
	if c.t.AtEnd() || !c.flags.extensionsEnabled || c.t.Peek(1).IsNot(tokenizer.OptionalToken) {
		return nil
	}

	// m1 is the address of the BranchFalse instruction whose target we don't know
	// yet — we'll patch it once we know where the "else" value starts.
	m1 := c.b.Mark()
	c.b.Emit(bytecode.BranchFalse, 0)

	// Consume the "?" and compile the "true" value expression.
	c.t.Advance(1)

	if err := c.logicalOr(); err != nil {
		return err
	}

	if c.t.AtEnd() || c.t.Peek(1).IsNot(tokenizer.ColonToken) {
		return c.compileError(errors.ErrMissingColon)
	}

	// m2 is the address of the unconditional Branch that jumps over the "false"
	// value after the "true" value has been evaluated.
	m2 := c.b.Mark()
	c.b.Emit(bytecode.Branch, 0)

	// Patch the BranchFalse (m1) to jump to the start of the "false" value.
	_ = c.b.SetAddressHere(m1)
	c.t.Advance(1) // consume the ":"

	if err := c.logicalOr(); err != nil {
		return err
	}

	// Patch the Branch (m2) to jump past the "false" value.
	_ = c.b.SetAddressHere(m2)

	return nil
}

// logicalAnd compiles the "&&" (boolean AND) operator with short-circuit
// evaluation. Short-circuit means: if the left operand is already false,
// the right operand is never evaluated.
//
// Implementation: the left operand is evaluated and duplicated on the stack.
// A BranchFalse instruction jumps over the right operand evaluation when the
// left side is false, leaving the (false) duplicate on the stack as the result.
// When the left side is true the duplicate is consumed by the And instruction.
func (c *Compiler) logicalAnd() error {
	if err := c.relations(); err != nil {
		return err
	}

	for c.t.IsNext(tokenizer.BooleanAndToken) {
		// Duplicate the left operand so BranchFalse can consume one copy while
		// leaving the other as the short-circuit result.
		c.b.Emit(bytecode.Dup)

		mark := c.b.Mark()
		c.b.Emit(bytecode.BranchFalse, 0)

		if err := c.relations(); err != nil {
			return err
		}

		c.b.Emit(bytecode.And)
		_ = c.b.SetAddressHere(mark)
	}

	return nil
}

// logicalOr compiles the "||" (boolean OR) operator with short-circuit
// evaluation. Short-circuit means: if the left operand is already true,
// the right operand is never evaluated.
//
// Implementation: the left operand is evaluated and duplicated on the stack.
// A BranchTrue instruction jumps over the right operand evaluation when the
// left side is true, leaving the (true) duplicate on the stack as the result.
// When the left side is false the duplicate is consumed by the Or instruction.
func (c *Compiler) logicalOr() error {
	if err := c.logicalAnd(); err != nil {
		return err
	}

	for c.t.IsNext(tokenizer.BooleanOrToken) {
		// Duplicate for the same reason as logicalAnd above.
		c.b.Emit(bytecode.Dup)

		mark := c.b.Mark()
		c.b.Emit(bytecode.BranchTrue, 0)

		if err := c.logicalAnd(); err != nil {
			return err
		}

		c.b.Emit(bytecode.Or)
		_ = c.b.SetAddressHere(mark)
	}

	return nil
}
