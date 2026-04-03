package compiler

import (
	bc "github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// functionCall compiles the argument list of a function call. The caller is
// responsible for having already consumed the opening "(" token. This function
// therefore starts reading argument expressions, separated by commas, until the
// closing ")" is found.
//
// The "..." variadic-flatten operator is also supported: when "..." follows an
// argument expression, a Flatten bytecode is emitted that tells the runtime to
// spread the argument (which must be a slice) into individual arguments.
//
// After all arguments are compiled, a Call instruction is emitted with the
// argument count as its operand. The runtime uses this count to pop the right
// number of values off the evaluation stack to pass to the function.
func (c *Compiler) functionCall() error {
	argc := 0 // number of argument expressions compiled so far

	for c.t.Peek(1).IsNot(tokenizer.EndOfListToken) {
		// Compile the next argument expression onto the stack.
		if err := c.conditional(); err != nil {
			return err
		}

		argc = argc + 1

		if c.t.AtEnd() {
			break
		}

		if c.t.Peek(1).Is(tokenizer.EndOfListToken) {
			break
		}

		// The "..." operator flattens a slice into individual arguments.
		// After a flatten, no more arguments can follow in the same call.
		if c.t.IsNext(tokenizer.VariadicToken) {
			c.b.Emit(bc.Flatten)

			break
		}

		if c.t.Peek(1).IsNot(tokenizer.CommaToken) {
			return c.compileError(errors.ErrInvalidList)
		}

		c.t.Advance(1)
	}

	// The argument list must be closed by ")".
	if c.t.AtEnd() || c.t.Peek(1).IsNot(tokenizer.EndOfListToken) {
		return c.compileError(errors.ErrMissingParenthesis)
	}

	c.t.Advance(1)

	// Emit the Call instruction. The runtime will pop argc values from the
	// stack and pass them to the function whose value is just below them.
	c.b.Emit(bc.Call, argc)

	return nil
}

// functionOrReference compiles either a plain reference (variable, member
// access, array index) or a function call. It first parses the base reference
// via reference(), then checks whether the next token is "(". If it is, the
// opening parenthesis is consumed and functionCall() is invoked to compile the
// argument list. Otherwise the parsed reference is left as the result.
func (c *Compiler) functionOrReference() error {
	if err := c.reference(); err != nil {
		return err
	}

	// If "(" follows, this is a function call expression.
	if c.t.IsNext(tokenizer.StartOfListToken) {
		return c.functionCall()
	}

	return nil
}
