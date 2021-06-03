package compiler

import (
	"github.com/tucats/ego/bytecode"
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
		if op == "==" || op == "!=" || op == "<" || op == "<=" || op == ">" || op == ">=" {
			c.t.Advance(1)

			err := c.addSubtract()
			if !errors.Nil(err) {
				return err
			}

			switch op {
			case "==":
				c.b.Emit(bc.Equal)

			case "!=":
				c.b.Emit(bc.NotEqual)

			case "<":
				c.b.Emit(bc.LessThan)

			case "<=":
				c.b.Emit(bc.LessThanOrEqual)

			case ">":
				c.b.Emit(bc.GreaterThan)

			case ">=":
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

		if c.t.IsNext("&&") {
			// Handle short-circuit from boolean
			c.b.Emit(bytecode.Dup)

			mark := c.b.Mark()
			c.b.Emit(bytecode.BranchFalse, 0)

			err := c.multDivide()
			if !errors.Nil(err) {
				return err
			}

			c.b.Emit(bytecode.And)
			_ = c.b.SetAddressHere(mark)

			continue
		}

		op := c.t.Peek(1)
		if util.InList(op, "+", "-", "&") {
			c.t.Advance(1)

			if c.t.IsNext(tokenizer.EndOfTokens) {
				return c.newError(errors.ErrMissingTerm)
			}

			err := c.multDivide()
			if !errors.Nil(err) {
				return err
			}

			switch op {
			case "+":
				c.b.Emit(bc.Add)

			case "-":
				c.b.Emit(bc.Sub)

			case "&":
				c.b.Emit(bc.And)
			}
		} else {
			parsing = false
		}
	}

	return nil
}

// multDivide compiles an expression containing "*", "^", "|", or "/" operators.
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
		if c.t.Peek(1) == "*" && tokenizer.IsSymbol(c.t.Peek(2)) && c.t.Peek(3) == "=" {
			parsing = false

			continue
		}

		if c.t.IsNext("||") {
			// Handle short-circuit from boolean
			c.b.Emit(bytecode.Dup)

			mark := c.b.Mark()
			c.b.Emit(bytecode.BranchTrue, 0)

			err := c.multDivide()
			if !errors.Nil(err) {
				return err
			}

			c.b.Emit(bytecode.Or)
			_ = c.b.SetAddressHere(mark)

			continue
		}

		if c.t.AnyNext("^", "*", "/", "|") {
			if c.t.IsNext(tokenizer.EndOfTokens) {
				return c.newError(errors.ErrMissingTerm)
			}

			err := c.unary()
			if !errors.Nil(err) {
				return err
			}

			switch op {
			case "^":
				c.b.Emit(bc.Exp)

			case "*":
				c.b.Emit(bc.Mul)

			case "/":
				c.b.Emit(bc.Div)

			case "|":
				c.b.Emit(bc.Or)
			}
		} else {
			parsing = false
		}
	}

	return nil
}
