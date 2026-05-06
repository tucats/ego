package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileBlock compiles a brace-enclosed statement block. The caller must have
// already consumed the opening "{" token before calling this function.
//
// A block introduces a new symbol scope. Any variables declared with ":=" or
// "var" inside the block are local to that block and are discarded when the
// block exits. This is implemented by emitting a PushScope bytecode at the
// start and a PopScope at the end; the runtime interpreter creates and destroys
// a child symbol table to match.
//
// Semicolons between statements are silently consumed. If the token stream ends
// before a closing "}" is found, a compile error is returned.
func (c *Compiler) compileBlock(runDefers bool) error {
	parsing := true
	c.blockDepth++

	// Tell the symbol-usage tracker that we are entering a new scope so it
	// can detect variables that are declared but never read.
	c.PushSymbolScope()

	// At runtime, PushScope creates a child symbol table so that variables
	// declared inside this block shadow but do not overwrite outer variables.
	c.b.Emit(bytecode.PushScope)

	for parsing {
		// A closing "}" ends the block.
		if c.t.IsNext(tokenizer.BlockEndToken) {
			break
		}

		if err := c.compileStatement(); err != nil {
			return err
		}

		// The tokenizer may insert semicolons between statements; skip them.
		_ = c.t.IsNext(tokenizer.SemicolonToken)

		// If we run out of tokens without a "}", the source is malformed.
		if c.t.AtEnd() {
			return c.compileError(errors.ErrMissingEndOfBlock)
		}
	}

	// If this block supports `defer` statements, run them now as we exit
	// the block.
	if runDefers {
		c.b.Emit(bytecode.RunDefers)
	}

	// Emit the matching PopScope so the runtime destroys the child symbol
	// table when execution leaves the block.
	c.b.Emit(bytecode.PopScope)

	c.blockDepth--

	// PopSymbolScope checks for any variables that were declared but never
	// referenced, reporting them as errors when unused-variable checking is on.
	return c.PopSymbolScope()
}

// compileRequiredBlock compiles a block that must be present. Unlike
// compileBlock, this function consumes the opening "{" itself (or accepts
// an empty-block token "{}"). If neither is found, a compile error is
// returned.
//
// IF this is a function block, runDefers will be true and causes the exit
// from the block to emit RunDefers which runs any pending defers for this
// call frame.
//
// This helper is used by if, for, func, switch, try, and other statements
// that are always followed by a block body.
func (c *Compiler) compileRequiredBlock(runDefers bool) error {
	// An empty block ({}) is legal and generates no code.
	if c.t.IsNext(tokenizer.EmptyBlockToken) {
		return nil
	}

	// A non-empty block must start with "{".
	if !c.t.IsNext(tokenizer.BlockBeginToken) {
		return c.compileError(errors.ErrMissingBlock)
	}

	return c.compileBlock(runDefers)
}
