package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileSwitch compiles a switch statement.
func (c *Compiler) compileSwitch() error {
	var (
		defaultBlock        *bytecode.ByteCode
		fallThrough         int
		conditional         bool
		hasScope            bool
		next                int
		switchTestValueName string
		err                 error
		fixups              = make([]int, 0)
	)

	if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
		return c.error(errors.ErrMissingExpression)
	}

	// The switch value cannot contain a struct initializer
	// that doesn't include a dereference after it. This
	// prevents getting tangled up by the switch syntax which
	// can look like a struct initializer.
	c.flags.disallowStructInits = true
	defer func() {
		c.flags.disallowStructInits = false
	}()

	if c.t.Peek(1) == tokenizer.BlockBeginToken {
		conditional = true
	} else {
		var err error

		// Do we have a symbol to store the value?
		switchTestValueName, hasScope, err = c.compileSwitchAssignedValue()
		if err != nil {
			return err
		}
	}

	// Switch statement is followed by block syntax, so look for the
	// start of block.
	if !c.t.IsNext(tokenizer.BlockBeginToken) {
		return c.error(errors.ErrMissingBlock)
	}

	// Iterate over each case or default selector in the switch block.
	for !c.t.IsNext(tokenizer.BlockEndToken) {
		if next > 0 {
			_ = c.b.SetAddressHere(next)
		}

		// Could be a default statement:
		if c.t.IsNext(tokenizer.DefaultToken) {
			defaultBlock, err = c.compileSwitchDefaultBlock()
			if err != nil {
				return err
			}
		} else {
			// Compile a case selector
			fixups, err = c.compileSwitchCase(conditional, switchTestValueName, &next, &fallThrough, fixups)
			if err != nil {
				return err
			}
		}
	}

	// If there was a last case with conditional, branch it here.
	if next > 0 {
		_ = c.b.SetAddressHere(next)
	}

	// If there was a default block, emit it here
	if defaultBlock != nil {
		c.b.Append(defaultBlock)
	}

	// Fixup all the jumps to the exit point
	for _, n := range fixups {
		_ = c.b.SetAddressHere(n)
	}

	// If we weren't using conditional cases, clean up the symbol used for
	// the value used for case matching. If we were given one by the source
	// code, we can just delete the scope. Otherwise, it was a unique
	// generated symbol name and code is emitted to delete it.
	if !conditional {
		if hasScope {
			c.b.Emit(bytecode.PopScope)
		} else {
			c.b.Emit(bytecode.SymbolDelete, switchTestValueName)
		}
	}

	return nil
}

func (c *Compiler) compileSwitchCase(conditional bool, switchTestValueName string, next *int, fallThrough *int, fixups []int) ([]int, error) {
	var err error

	if !c.t.IsNext(tokenizer.CaseToken) {
		return nil, errors.ErrMissingCase
	}

	if c.t.IsNext(tokenizer.ColonToken) {
		return nil, errors.ErrMissingColon
	}

	if err := c.emitExpression(); err != nil {
		return nil, err
	}

	if !conditional {
		c.b.Emit(bytecode.Load, switchTestValueName)
		c.b.Emit(bytecode.Equal)
	}

	*next = c.b.Mark()

	c.b.Emit(bytecode.BranchFalse, 0)

	if !c.t.IsNext(tokenizer.ColonToken) {
		return nil, err
	}

	if *fallThrough > 0 {
		if err := c.b.SetAddressHere(*fallThrough); err != nil {
			return nil, err
		}

		*fallThrough = 0
	}

	for !tokenizer.InList(c.t.Peek(1),
		tokenizer.CaseToken,
		tokenizer.DefaultToken,
		tokenizer.FallthroughToken,
		tokenizer.BlockEndToken) {
		if err := c.compileStatement(); err != nil {
			return nil, err
		}
	}

	if c.t.IsNext(tokenizer.FallthroughToken) {
		c.t.IsNext(tokenizer.SemicolonToken)

		*fallThrough = c.b.Mark()

		c.b.Emit(bytecode.Branch, 0)
	} else {
		fixups = append(fixups, c.b.Mark())

		c.b.Emit(bytecode.Branch, 0)
	}

	return fixups, nil
}

func (c *Compiler) compileSwitchDefaultBlock() (*bytecode.ByteCode, error) {
	var defaultBlock *bytecode.ByteCode

	if !c.t.IsNext(tokenizer.ColonToken) {
		return nil, c.error(errors.ErrMissingColon)
	}

	savedBC := c.b
	c.b = bytecode.New("default switch")

	for c.t.Peek(1) != tokenizer.CaseToken && c.t.Peek(1) != tokenizer.BlockEndToken {
		if err := c.compileStatement(); err != nil {
			return nil, err
		}
	}

	defaultBlock = c.b
	c.b = savedBC

	return defaultBlock, nil
}

func (c *Compiler) compileSwitchAssignedValue() (string, bool, error) {
	var (
		hasScope            bool
		switchTestValueName string
	)

	if c.t.Peek(1).IsIdentifier() && c.t.Peek(2).IsToken(tokenizer.DefineToken) {
		switchTestValueName = c.t.Next().Spelling()
		hasScope = true

		c.b.Emit(bytecode.PushScope)
		c.t.Advance(1)
	} else {
		switchTestValueName = data.GenerateName()
	}

	if err := c.emitExpression(); err != nil {
		return "", false, err
	}

	if c.flags.hasUnwrap {
		c.b.Emit(bytecode.CreateAndStore, switchTestValueName)

		switchTestValueName = data.GenerateName()
		c.b.Emit(bytecode.CreateAndStore, switchTestValueName)
	} else {
		c.b.Emit(bytecode.CreateAndStore, switchTestValueName)
	}

	c.flags.disallowStructInits = false
	c.flags.hasUnwrap = false

	return switchTestValueName, hasScope, nil
}
