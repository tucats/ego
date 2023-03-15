package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileSwitch compiles a switch statement.
func (c *Compiler) compileSwitch() error {
	var defaultBlock *bytecode.ByteCode

	if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
		return c.error(errors.ErrMissingExpression)
	}

	fallThrough := 0
	conditional := false
	hasScope := false

	next := 0
	fixups := make([]int, 0)
	switchTestValueName := ""

	// The switch value cannot contain a struct initializer
	// that doesn't include a derefernce after it. This
	// prevents getting tangled up by the switch syntax which
	// can look like a struct initializer.
	c.flags.disallowStructInits = true

	if c.t.Peek(1) == tokenizer.BlockBeginToken {
		conditional = true
	} else {
		// Do we have a symbol to store the value?
		if c.t.Peek(1).IsIdentifier() && c.t.Peek(2).IsToken(tokenizer.DefineToken) {
			switchTestValueName = c.t.Next().Spelling()
			hasScope = true

			c.b.Emit(bytecode.PushScope)
			c.t.Advance(1)
		} else {
			switchTestValueName = data.GenerateName()
		}

		// Parse the expression to test
		if err := c.emitExpression(); err != nil {
			return err
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
			if !c.t.IsNext(tokenizer.ColonToken) {
				return c.error(errors.ErrMissingColon)
			}

			savedBC := c.b
			c.b = bytecode.New("default switch")

			for c.t.Peek(1) != tokenizer.CaseToken && c.t.Peek(1) != tokenizer.BlockEndToken {
				err := c.compileStatement()
				if err != nil {
					return err
				}
			}

			defaultBlock = c.b
			c.b = savedBC
		} else {
			// Must be a "case" statement:
			if !c.t.IsNext(tokenizer.CaseToken) {
				return c.error(errors.ErrMissingCase)
			}

			if c.t.IsNext(tokenizer.ColonToken) {
				return c.error(errors.ErrMissingExpression)
			}

			if err := c.emitExpression(); err != nil {
				return err
			}

			// If it was't a conditional switch, test for the
			// specific value in the assigned variable. If it was
			// a conditional mode statement, the case expression must
			// be evaluated as a boolean expression.
			if !conditional {
				c.b.Emit(bytecode.Load, switchTestValueName)
				c.b.Emit(bytecode.Equal)
			}

			next = c.b.Mark()

			c.b.Emit(bytecode.BranchFalse, 0)

			if !c.t.IsNext(tokenizer.ColonToken) {
				return c.error(errors.ErrMissingColon)
			}

			if fallThrough > 0 {
				if err := c.b.SetAddressHere(fallThrough); err != nil {
					return c.error(err)
				}

				fallThrough = 0
			}

			// Compile all the statements in the selector, stopping when
			// we hit the next case, default, fallthrough, or end of the
			// set of selectors.
			for !tokenizer.InList(c.t.Peek(1),
				tokenizer.CaseToken,
				tokenizer.DefaultToken,
				tokenizer.FallthroughToken,
				tokenizer.BlockEndToken) {
				err := c.compileStatement()
				if err != nil {
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
