package compiler

import (
	"github.com/tucats/ego/bytecode"
	bc "github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// findDeferCallEnd returns the token-stream position immediately after the
// closing ')' of the call expression that starts at startPos.  It skips the
// leading identifier chain (e.g. "wg.Done") and then counts parentheses to
// find the matching close of the argument list.
func (c *Compiler) findDeferCallEnd(startPos int) int {
	tokens := c.t.Tokens
	pos := startPos
	n := len(tokens)

	// Skip identifier tokens and dots (e.g. "wg.Done").
	for pos < n {
		t := tokens[pos]
		if t.IsIdentifier() || t.Is(tokenizer.DotToken) {
			pos++
		} else {
			break
		}
	}

	// Count parentheses to locate the matching close of the argument list.
	depth := 0

	for pos < n {
		t := tokens[pos]
		pos++

		if t.Is(tokenizer.StartOfListToken) {
			depth++
		} else if t.Is(tokenizer.EndOfListToken) {
			depth--
			if depth == 0 {
				return pos
			}
		}
	}

	return pos
}

// compileDefer compiles the "defer" statement. This compiles a statement,
// and attaches the resulting bytecode to the compilation unit's defer queue.
// Later, when a return is processed, this queue will be used to generate the
// appropriate deferred operations. The order of the "defer" statements determines
// the order in the queue, and therefore the order in which they are run when a
// return is executed.
func (c *Compiler) compileDefer() error {
	minDepth := 1
	if c.flags.exitEnabled {
		minDepth = 2
	}

	if c.functionDepth < minDepth {
		return c.compileError(errors.ErrDeferOutsideFunction)
	}

	if c.t.EndOfStatement() {
		return c.compileError(errors.ErrMissingFunction)
	}

	// Is it a function constant?
	if c.t.IsNext(tokenizer.FuncToken) {
		c.b.Emit(bc.DeferStart, true)

		// Compile a function literal onto the stack.
		if err := c.compileFunctionDefinition(c.isLiteralFunction()); err != nil {
			return err
		}
	} else {
		c.b.Emit(bc.DeferStart, true)

		// Peek ahead to see if this is a legit function call. If the next token is not an
		// identifier, and it's not followed by a parenthesis or dot-notation identifier,
		// then this is not a function call and we're done.
		if !c.t.Peek(1).IsIdentifier() || (c.t.Peek(2).IsNot(tokenizer.StartOfListToken) && c.t.Peek(2).IsNot(tokenizer.DotToken)) {
			return c.compileError(errors.ErrInvalidFunctionCall)
		}

		// Wrap the call in an anonymous function literal so the full call — including
		// receiver setup (LoadThis / "__this") — is deferred rather than just the
		// resolved function pointer.  Logically:  defer f()  →  defer func(){ f() }()
		startPos := c.t.Mark()
		endPos := c.findDeferCallEnd(startPos)

		// Insert '}()' after the call expression.  TokenP < endPos so it is not adjusted.
		c.t.Insert(endPos, tokenizer.BlockEndToken, tokenizer.StartOfListToken, tokenizer.EndOfListToken)

		// Insert 'func(){' before the call.  Insert advances TokenP past the new tokens,
		// so reset it back to startPos so the compiler reads from 'func'.
		c.t.Insert(startPos, tokenizer.FuncToken, tokenizer.StartOfListToken, tokenizer.EndOfListToken, tokenizer.BlockBeginToken)
		c.t.Set(startPos)

		// Consume the injected 'func' token and compile the anonymous literal.
		c.t.IsNext(tokenizer.FuncToken)

		if err := c.compileFunctionDefinition(c.isLiteralFunction()); err != nil {
			return err
		}
	}

	// Let's stop now and see if the stack looks right.
	lastBytecode := c.b.Mark()

	i := c.b.Instruction(lastBytecode - 1)
	if i.Operation != bytecode.Call {
		return c.compileError(errors.ErrInvalidFunctionCall)
	}

	argc, err := data.Int(i.Operand)
	if err != nil {
		return c.compileError(err)
	}

	// Drop the Call operation from the end of the bytecode
	// and replace with the "defer function" operation.
	c.b.Delete(lastBytecode - 1)
	c.b.Emit(bc.Defer, argc)

	return nil
}
