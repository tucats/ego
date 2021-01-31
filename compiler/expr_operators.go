package compiler

import (
	bc "github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// relations compiles a relationship expression.
func (c *Compiler) relations() error {
	err := c.addSubtract()
	if err != nil {
		return err
	}
	var parsing = true
	for parsing {
		if c.t.AtEnd() {
			break
		}
		op := c.t.Peek(1)
		if op == "==" || op == "!=" || op == "<" || op == "<=" || op == ">" || op == ">=" {
			c.t.Advance(1)
			err := c.addSubtract()
			if err != nil {
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

// addSubtract commpiles an expression containing "+", "&", or "-" operators
func (c *Compiler) addSubtract() error {
	err := c.multDivide()
	if err != nil {
		return err
	}
	var parsing = true
	for parsing {
		if c.t.AtEnd() {
			break
		}
		op := c.t.Peek(1)
		if util.InList(op, "+", "-", "&") {
			c.t.Advance(1)
			if c.t.IsNext(tokenizer.EndOfTokens) {
				return c.NewError(MissingTermError)
			}
			err := c.multDivide()
			if err != nil {
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
func (c *Compiler) multDivide() error {
	err := c.unary()
	if err != nil {
		return err
	}
	var parsing = true
	for parsing {
		if c.t.AtEnd() {
			break
		}
		op := c.t.Peek(1)
		if c.t.AnyNext("^", "*", "/", "|") {
			if c.t.IsNext(tokenizer.EndOfTokens) {
				return c.NewError(MissingTermError)
			}
			err := c.unary()
			if err != nil {
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
