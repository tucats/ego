package compiler

import (
	bc "github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// relations compiles a comparison expression. It first parses the left operand
// via addSubtract, then checks whether the next token is a comparison operator.
// If it is, the right operand is also parsed via addSubtract and the appropriate
// bytecode instruction is emitted. This loop continues so that chains like
// a == b != c are handled (though Ego does not short-circuit comparisons the
// way &&/|| do).
//
// Comparison operators handled: ==, !=, <, <=, >, >=.
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

			// Emit the bytecode instruction that corresponds to the operator.
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

// addSubtract compiles an additive expression. It first parses the left operand
// via multiplyDivide (which has higher precedence), then checks for "+", "-",
// "|" (bitwise OR), "<<" (left shift), or ">>" (right shift) operators in a loop.
//
// For each operator found, the right operand is parsed by multiplyDivide and the
// appropriate bytecode instruction is emitted. The loop continues so that chains
// like a + b - c + d are handled left-to-right.
//
// A missing right-hand term after an operator is treated as a compile error.
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

			// Left shift is encoded as Negate + BitShift so that the runtime
			// instruction can distinguish shift direction from a single operand.
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

// multiplyDivide compiles a multiplicative expression. It first parses the
// left operand via unary, then checks for "*", "/", "%", "^" (exponentiation),
// or "&" (bitwise AND) operators in a loop.
//
// Special case: the token sequence "* identifier =" is NOT a multiply; it is
// the start of a pointer-dereference assignment (e.g. *ptr = value). When that
// pattern is detected the loop exits immediately so the assignment compiler
// can handle it.
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

		// Guard: "* identifier =" is a pointer dereference assignment, not a multiply.
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
