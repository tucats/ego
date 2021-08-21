package compiler

import (
	"github.com/tucats/ego/bytecode"
	bc "github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// reference parses a structure or array reference.
func (c *Compiler) reference() *errors.EgoError {
	// Parse the function call or exprssion atom
	err := c.expressionAtom()
	if !errors.Nil(err) {
		return err
	}

	lastName := ""
	parsing := true
	// is there a trailing structure or array reference?
	for parsing && !c.t.AtEnd() {
		op := c.t.Peek(1)

		switch op {
		// Structure initialization
		case "{":
			name := c.t.Peek(2)
			colon := c.t.Peek(3)

			if tokenizer.IsSymbol(name) && colon == ":" {
				c.b.Emit(bc.Push, datatypes.TypeMDKey)

				err := c.expressionAtom()
				if !errors.Nil(err) {
					return err
				}

				i := c.b.Opcodes()
				ix := i[len(i)-1]
				ix.Operand = datatypes.GetInt(ix.Operand) + 1 // __type
				i[len(i)-1] = ix
			} else {
				parsing = false

				break
			}
		// Function invocation
		case "(":
			c.t.Advance(1)

			err := c.functionCall()
			if !errors.Nil(err) {
				return err
			}

		// Map member reference
		case ".":
			c.t.Advance(1)

			lastName = c.t.Next()
			if !tokenizer.IsSymbol(lastName) {
				return c.newError(errors.ErrInvalidIdentifier)
			}

			lastName = c.normalize(lastName)

			// Peek ahead. is this a chained call? If so, set the This
			// value
			if c.t.Peek(1) == "(" {
				c.b.Emit(bc.SetThis)
			}

			c.b.Emit(bc.Member, lastName)

			if c.t.IsNext("{}") {
				c.b.Emit(bytecode.Load, "new")
				c.b.Emit(bytecode.Swap)
				c.b.Emit(bytecode.Call, 1)
			} else {
				// Is it a generator for a type?
				if c.t.Peek(1) == "{" && tokenizer.IsSymbol(c.t.Peek(2)) && c.t.Peek(3) == ":" {
					c.b.Emit(bytecode.Push, datatypes.TypeMDKey)

					err := c.expressionAtom()
					if !errors.Nil(err) {
						return err
					}

					i := c.b.Opcodes()
					ix := i[len(i)-1]
					ix.Operand = datatypes.GetInt(ix.Operand) + 1 // __type and
					i[len(i)-1] = ix

					return nil
				}
			}

		// Array index reference
		case "[":
			c.t.Advance(1)

			err := c.conditional()
			if !errors.Nil(err) {
				return err
			}

			// is it a slice instead of an index?
			if c.t.IsNext(":") {
				err := c.conditional()
				if !errors.Nil(err) {
					return err
				}

				c.b.Emit(bc.LoadSlice)

				if c.t.Next() != "]" {
					return c.newError(errors.ErrMissingBracket)
				}
			} else {
				// Nope, singular index
				if c.t.Next() != "]" {
					return c.newError(errors.ErrMissingBracket)
				}

				c.b.Emit(bc.LoadIndex)
			}

		// Nothing else, term is complete
		default:
			return nil
		}
	}

	return nil
}
