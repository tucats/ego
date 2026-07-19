package compiler

import (
	bc "github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/tokenizer"
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
//
// hasReceiver is captured from c.flags.pendingReceiverCall and immediately
// reset, *before* any argument expressions are compiled, because those
// argument expressions can themselves contain nested "X.Y(...)" calls that
// set and consume the same flag for their own, unrelated, call (see CALL-11
// in docs/ISSUES.md). Capturing it up front, rather than re-reading it after
// compiling arguments, ensures this call's own Call instruction reports
// whether *this* call had a receiver pushed for it, not whatever the flag
// happened to hold after the last nested call resolved.
func (c *Compiler) functionCall() error {
	hasReceiver := c.flags.pendingReceiverCall
	c.flags.pendingReceiverCall = false

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
	//
	// When hasReceiver is true, the operand is the two-element form
	// []any{argc, true} instead of a bare argc, telling callByteCode that a
	// SetThis was emitted for this call and it must consume exactly one entry
	// from the receiver stack before dispatching (see CALL-11 in
	// docs/ISSUES.md). Every other call site that emits a Call instruction
	// (macro invocations, defaults, defer replay, etc.) continues to use the
	// bare-argc form, which callByteCode treats as "no receiver pending" --
	// exactly today's behavior.
	if hasReceiver {
		c.b.Emit(bc.Call, []any{argc, true})
	} else {
		c.b.Emit(bc.Call, argc)
	}

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
