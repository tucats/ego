package compiler

import (
	bc "github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// relations compiles a relationship expression.
func (c *Compiler) relations() error {
	if err := c.addSubtract(); err != nil {
		return err
	}

	parsing := true
	for parsing {
		if c.t.AtEnd() {
			break
		}

		op := c.t.Peek(1)

		if tokenizer.InList(op,
			tokenizer.EqualsToken,
			tokenizer.NotEqualsToken,
			tokenizer.LessThanToken,
			tokenizer.LessThanOrEqualsToken,
			tokenizer.GreaterThanToken,
			tokenizer.GreaterThanOrEqualsToken) {
			c.t.Advance(1)

			if err := c.addSubtract(); err != nil {
				return err
			}

			switch {
			case op.Is(tokenizer.EqualsToken):
				c.b.Emit(bc.Equal)

			case op.Is(tokenizer.NotEqualsToken):
				c.b.Emit(bc.NotEqual)

			case op.Is(tokenizer.LessThanToken):
				c.b.Emit(bc.LessThan)

			case op.Is(tokenizer.LessThanOrEqualsToken):
				c.b.Emit(bc.LessThanOrEqual)

			case op.Is(tokenizer.GreaterThanToken):
				c.b.Emit(bc.GreaterThan)

			case op.Is(tokenizer.GreaterThanOrEqualsToken):
				c.b.Emit(bc.GreaterThanOrEqual)
			}
		} else {
			parsing = false
		}
	}

	return nil
}

// addSubtract compiles an expression containing "+", "&", or "-" operators.
func (c *Compiler) addSubtract() error {
	if err := c.multiplyDivide(); err != nil {
		return err
	}

	parsing := true
	for parsing {
		if c.t.AtEnd() {
			break
		}

		op := c.t.Peek(1)
		if tokenizer.InList(op, tokenizer.AddToken,
			tokenizer.SubtractToken,
			tokenizer.OrToken,
			tokenizer.ShiftLeftToken,
			tokenizer.ShiftRightToken) {
			c.t.Advance(1)

			if c.t.IsNext(tokenizer.EndOfTokens) {
				return c.compileError(errors.ErrMissingTerm)
			}

			if err := c.multiplyDivide(); err != nil {
				return err
			}

			switch {
			case op.Is(tokenizer.AddToken):
				c.b.Emit(bc.Add)

			case op.Is(tokenizer.SubtractToken):
				c.b.Emit(bc.Sub)

			case op.Is(tokenizer.OrToken):
				c.b.Emit(bc.BitOr)

			case op.Is(tokenizer.ShiftLeftToken):
				c.b.Emit(bc.Negate)
				c.b.Emit(bc.BitShift)

			case op.Is(tokenizer.ShiftRightToken):
				c.b.Emit(bc.BitShift)
			}
		} else {
			parsing = false
		}
	}

	return nil
}

// multiplyDivide compiles an expression containing "*", "^", "|", "%" or "/" operators.
func (c *Compiler) multiplyDivide() error {
	if err := c.unary(); err != nil {
		return err
	}

	parsing := true
	for parsing {
		if c.t.AtEnd() {
			break
		}

		op := c.t.Peek(1)

		// Special case; if the next tokens are * <symbol> = then this isn't a multiply,
		// but rather a pointer dereference assignment statement boundary.
		if c.t.Peek(1).Is(tokenizer.PointerToken) && c.t.Peek(2).IsIdentifier() && c.t.Peek(3).Is(tokenizer.AssignToken) {
			parsing = false

			continue
		}

		if c.t.AnyNext(
			tokenizer.ExponentToken,
			tokenizer.MultiplyToken,
			tokenizer.DivideToken,
			tokenizer.AndToken,
			tokenizer.ModuloToken,
		) {
			if c.t.IsNext(tokenizer.EndOfTokens) {
				return c.compileError(errors.ErrMissingTerm)
			}

			if err := c.unary(); err != nil {
				return err
			}

			switch {
			case op.Is(tokenizer.ExponentToken):
				c.b.Emit(bc.Exp)

			case op.Is(tokenizer.MultiplyToken):
				c.b.Emit(bc.Mul)

			case op.Is(tokenizer.DivideToken):
				c.b.Emit(bc.Div)

			case op.Is(tokenizer.AndToken):
				c.b.Emit(bc.BitAnd)

			case op.Is(tokenizer.ModuloToken):
				c.b.Emit(bc.Modulo)
			}
		} else {
			parsing = false
		}
	}

	return nil
}
