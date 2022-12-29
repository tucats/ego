package compiler

import (
	bc "github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// relations compiles a relationship expression.
func (c *Compiler) relations() *errors.EgoError {
	err := c.addSubtract()
	if !errors.Nil(err) {
		return err
	}

	parsing := true
	for parsing {
		if c.t.AtEnd() {
			break
		}

		op := c.t.Peek(1)

		if util.InList(op,
			tokenizer.EqualsToken,
			tokenizer.NotEqualsToken,
			tokenizer.LessThanToken,
			tokenizer.LessThanOrEqualsToken,
			tokenizer.GreaterThanToken,
			tokenizer.GreaterThanOrEqualsToken) {
			c.t.Advance(1)

			err := c.addSubtract()
			if !errors.Nil(err) {
				return err
			}

			switch op {
			case tokenizer.EqualsToken:
				c.b.Emit(bc.Equal)

			case tokenizer.NotEqualsToken:
				c.b.Emit(bc.NotEqual)

			case tokenizer.LessThanToken:
				c.b.Emit(bc.LessThan)

			case tokenizer.LessThanOrEqualsToken:
				c.b.Emit(bc.LessThanOrEqual)

			case tokenizer.GreaterThanToken:
				c.b.Emit(bc.GreaterThan)

			case tokenizer.GreaterThanOrEqualsToken:
				c.b.Emit(bc.GreaterThanOrEqual)
			}
		} else {
			parsing = false
		}
	}

	return nil
}

// addSubtract commpiles an expression containing "+", "&", or "-" operators.
func (c *Compiler) addSubtract() *errors.EgoError {
	err := c.multDivide()
	if !errors.Nil(err) {
		return err
	}

	parsing := true
	for parsing {
		if c.t.AtEnd() {
			break
		}

		op := c.t.Peek(1)
		if util.InList(op, tokenizer.AddToken,
			tokenizer.SubtractToken,
			tokenizer.OrToken,
			tokenizer.ShiftLeftToken,
			tokenizer.ShiftRightToken) {
			c.t.Advance(1)

			if c.t.IsNext(tokenizer.EndOfTokens) {
				return c.newError(errors.ErrMissingTerm)
			}

			err := c.multDivide()
			if !errors.Nil(err) {
				return err
			}

			switch op {
			case tokenizer.AddToken:
				c.b.Emit(bc.Add)

			case tokenizer.SubtractToken:
				c.b.Emit(bc.Sub)

			case tokenizer.OrToken:
				c.b.Emit(bc.BitOr)

			case tokenizer.ShiftLeftToken:
				c.b.Emit(bc.Negate)
				c.b.Emit(bc.BitShift)

			case tokenizer.ShiftRightToken:
				c.b.Emit(bc.BitShift)
			}
		} else {
			parsing = false
		}
	}

	return nil
}

// multDivide compiles an expression containing "*", "^", "|", "%" or "/" operators.
func (c *Compiler) multDivide() *errors.EgoError {
	err := c.unary()
	if !errors.Nil(err) {
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
		if c.t.Peek(1) == tokenizer.PointerToken && tokenizer.IsSymbol(c.t.Peek(2)) && c.t.Peek(3) == tokenizer.AssignToken {
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
				return c.newError(errors.ErrMissingTerm)
			}

			err := c.unary()
			if !errors.Nil(err) {
				return err
			}

			switch op {
			case tokenizer.ExponentToken:
				c.b.Emit(bc.Exp)

			case tokenizer.MultiplyToken:
				c.b.Emit(bc.Mul)

			case tokenizer.DivideToken:
				c.b.Emit(bc.Div)

			case tokenizer.AndToken:
				c.b.Emit(bc.BitAnd)

			case tokenizer.ModuloToken:
				c.b.Emit(bc.Modulo)
			}
		} else {
			parsing = false
		}
	}

	return nil
}
