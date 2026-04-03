package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/tokenizer"
)

// unary handles unary prefix operators: arithmetic negation ("-") and logical
// NOT ("!"). It is recursive so that expressions like --x or !!b work
// correctly (each call strips one prefix operator).
//
// For arithmetic negation, a compile-time optimisation is applied: if the
// operand is a numeric literal that was just pushed by the immediately
// preceding instruction, the literal's value is negated in-place in the
// bytecode rather than emitting a separate Negate instruction. This avoids
// a runtime operation for the common case of negative literal constants such
// as -1 or -3.14.
//
// For all other operands (identifiers, expressions, etc.) an explicit
// Negate bytecode is emitted.
//
// If neither "-" nor "!" is present, the call is forwarded to
// functionOrReference which handles the higher-priority non-prefix operators.
func (c *Compiler) unary() error {
	t := c.t.Peek(1)

	switch {
	case t.Is(tokenizer.NegateToken):
		c.t.Advance(1)

		if err := c.unary(); err != nil {
			return err
		}

		// Constant-folding optimisation: if the last emitted instruction is
		// a Push of a numeric literal, flip its sign directly in the bytecode
		// rather than emitting a separate Negate operation.
		addr := c.b.Mark() - 1
		i := c.b.Instruction(c.b.Mark() - 1)

		if i.Operation == bytecode.Push {
			switch v := i.Operand.(type) {
			case byte:
				i.Operand = -v
				c.b.Opcodes()[addr] = *i

			case uint16:
				i.Operand = -v
				c.b.Opcodes()[addr] = *i

			case int8:
				i.Operand = -v
				c.b.Opcodes()[addr] = *i

			case uint32:
				i.Operand = -v
				c.b.Opcodes()[addr] = *i

			case uint:
				i.Operand = -v
				c.b.Opcodes()[addr] = *i

			case uint64:
				i.Operand = -v
				c.b.Opcodes()[addr] = *i

			case int32:
				i.Operand = -v
				c.b.Opcodes()[addr] = *i

			case int:
				i.Operand = -v
				c.b.Opcodes()[addr] = *i

			case int64:
				i.Operand = -v
				c.b.Opcodes()[addr] = *i

			case float32:
				i.Operand = -v
				c.b.Opcodes()[addr] = *i

			case float64:
				i.Operand = -v
				c.b.Opcodes()[addr] = *i

			default:
				c.b.Emit(bytecode.Negate, false)
			}
		} else {
			// Non-constant operand: emit a runtime Negate instruction.
			// The false operand tells the runtime this is arithmetic (not boolean) negation.
			c.b.Emit(bytecode.Negate, false)
		}

	case t.Is(tokenizer.NotToken):
		c.t.Advance(1)

		if err := c.unary(); err != nil {
			return err
		}

		// The true operand tells the runtime this is boolean (NOT) negation.
		c.b.Emit(bytecode.Negate, true)

	default:
		// No unary prefix — delegate to the next level of the grammar.
		return c.functionOrReference()
	}

	return nil
}
