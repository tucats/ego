package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// conditional handles parsing the ?: trinary operator. The first term is
// converted to a boolean value, and if true the second term is returned, else
// the third term. All terms must be present.
func (c *Compiler) conditional() *errors.EgoError {
	// Parse the conditional
	err := c.logicalOr()
	if !errors.Nil(err) {
		return err
	}

	// If this is not a conditional, we're done. Conditionals
	// are only permitted when extensions are enabled.
	if c.t.AtEnd() || !c.extensionsEnabled || c.t.Peek(1) != "?" {
		return nil
	}

	m1 := c.b.Mark()

	c.b.Emit(bytecode.BranchFalse, 0)

	// Parse both parts of the alternate values
	c.t.Advance(1)

	err = c.logicalOr()
	if !errors.Nil(err) {
		return err
	}

	if c.t.AtEnd() || c.t.Peek(1) != tokenizer.ColonToken {
		return c.newError(errors.ErrMissingColon)
	}

	m2 := c.b.Mark()

	c.b.Emit(bytecode.Branch, 0)
	_ = c.b.SetAddressHere(m1)
	c.t.Advance(1)

	err = c.logicalOr()
	if !errors.Nil(err) {
		return err
	}

	// Patch up the forward references.
	_ = c.b.SetAddressHere(m2)

	return nil
}

func (c *Compiler) logicalAnd() *errors.EgoError {
	err := c.relations()
	if !errors.Nil(err) {
		return err
	}

	for c.t.AnyNext("&&", "and") {
		// Handle short-circuit from boolean
		c.b.Emit(bytecode.Dup)

		mark := c.b.Mark()
		c.b.Emit(bytecode.BranchFalse, 0)

		err := c.relations()
		if !errors.Nil(err) {
			return err
		}

		c.b.Emit(bytecode.And)
		_ = c.b.SetAddressHere(mark)
	}

	return nil
}

func (c *Compiler) logicalOr() *errors.EgoError {
	err := c.logicalAnd()
	if !errors.Nil(err) {
		return err
	}

	for c.t.AnyNext("||", "or") {
		// Handle short-circuit from boolean
		c.b.Emit(bytecode.Dup)

		mark := c.b.Mark()
		c.b.Emit(bytecode.BranchTrue, 0)

		err := c.logicalAnd()
		if !errors.Nil(err) {
			return err
		}

		c.b.Emit(bytecode.Or)
		_ = c.b.SetAddressHere(mark)
	}

	return nil
}
