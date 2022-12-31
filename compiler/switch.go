package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileSwitch compiles a switch statement.
func (c *Compiler) compileSwitch() *errors.EgoError {
	var defaultBlock *bytecode.ByteCode

	if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
		return c.newError(errors.ErrMissingExpression)
	}

	fallThrough := 0
	next := 0
	fixups := make([]int, 0)
	t := datatypes.GenerateName()

	// The switch value cannot contain a struct initializer
	// that doesn't include a derefernce after it. This
	// prevents getting tangled up by the switch syntax which
	// can look like a struct initializer.
	c.flags.disallowStructInits = true

	// Parse the expression to test
	tx, err := c.Expression()
	c.flags.disallowStructInits = false

	if !errors.Nil(err) {
		return err
	}

	c.b.Append(tx)
	c.b.Emit(bytecode.SymbolCreate, t)
	c.b.Emit(bytecode.Store, t)

	if !c.t.IsNext(tokenizer.BlockBeginToken) {
		return c.newError(errors.ErrMissingBlock)
	}

	for !c.t.IsNext(tokenizer.BlockEndToken) {
		if next > 0 {
			_ = c.b.SetAddressHere(next)
		}

		// Could be a default statement:
		if c.t.IsNext(tokenizer.DefaultToken) {
			if !c.t.IsNext(tokenizer.ColonToken) {
				return c.newError(errors.ErrMissingColon)
			}

			savedBC := c.b
			c.b = bytecode.New("default switch")

			for c.t.Peek(1) != tokenizer.CaseToken && c.t.Peek(1) != tokenizer.BlockEndToken {
				err := c.compileStatement()
				if !errors.Nil(err) {
					return err
				}
			}

			defaultBlock = c.b
			c.b = savedBC
		} else {
			// Must be a "case" statement:
			if !c.t.IsNext(tokenizer.CaseToken) {
				return c.newError(errors.ErrMissingCase)
			}

			if c.t.IsNext(tokenizer.ColonToken) {
				return c.newError(errors.ErrMissingExpression)
			}

			cx, err := c.Expression()
			if !errors.Nil(err) {
				return err
			}

			c.b.Append(cx)
			c.b.Emit(bytecode.Load, t)
			c.b.Emit(bytecode.Equal)

			next = c.b.Mark()

			c.b.Emit(bytecode.BranchFalse, 0)

			if !c.t.IsNext(tokenizer.ColonToken) {
				return c.newError(errors.ErrMissingColon)
			}

			if fallThrough > 0 {
				if err := c.b.SetAddressHere(fallThrough); !errors.Nil(err) {
					return c.newError(err)
				}

				fallThrough = 0
			}

			for !tokenizer.InList(c.t.Peek(1),
				tokenizer.CaseToken,
				tokenizer.DefaultToken,
				tokenizer.FallthroughToken,
				tokenizer.BlockEndToken) {
				err := c.compileStatement()
				if !errors.Nil(err) {
					return err
				}
			}

			// Emit the code that will jump to the exit point of the statement,
			// Unless we're on a "fallthrough" statement, in which case we just
			// eat the token.
			if c.t.Peek(1) == tokenizer.FallthroughToken {
				c.t.Advance(1)
				fallThrough = c.b.Mark()
				c.b.Emit(bytecode.Branch, 0)
			} else {
				fixups = append(fixups, c.b.Mark())

				c.b.Emit(bytecode.Branch, 0)
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

	c.b.Emit(bytecode.SymbolDelete, t)

	return nil
}
