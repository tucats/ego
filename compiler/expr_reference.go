package compiler

import (
	bc "github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// reference parses a structure or array reference
func (c *Compiler) reference() error {
	// Parse the function call or exprssion atom
	err := c.expressionAtom()
	if err != nil {
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
				c.b.Emit(bc.Push, "__type")
				c.b.Emit(bc.LoadIndex)
				c.b.Emit(bc.Push, "__type")
				err := c.expressionAtom()
				if err != nil {
					return err
				}
				i := c.b.Opcodes()
				ix := i[len(i)-1]
				ix.Operand = util.GetInt(ix.Operand) + 1
				i[len(i)-1] = ix
			} else {
				parsing = false

				break
			}
		// Function invocation
		case "(":
			c.t.Advance(1)
			err := c.functionCall()
			if err != nil {
				return err
			}

		// Map member reference
		case ".":
			c.t.Advance(1)
			lastName = c.t.Next()
			c.b.Emit(bc.Push, lastName)
			c.b.Emit(bc.Member)

		// Array index reference
		case "[":
			c.t.Advance(1)
			err := c.conditional()
			if err != nil {
				return err
			}

			// is it a slice instead of an index?
			if c.t.IsNext(":") {
				err := c.conditional()
				if err != nil {
					return err
				}
				c.b.Emit(bc.LoadSlice)
				if c.t.Next() != "]" {
					return c.NewError(MissingBracketError)
				}
			} else {
				// Nope, singular index
				if c.t.Next() != "]" {
					return c.NewError(MissingBracketError)
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
