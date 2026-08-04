package compiler

import (
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// compileError generates a new compiler error. This includes storing information
// about the active package and source file being compiled as well as the source
// line/column info.
func (c *Compiler) compileError(err error, args ...any) *errors.Error {
	if c == nil || c.t == nil {
		return errors.New(err)
	}

	// Work out which token the message should point at.
	//
	// The tokenizer numbers its look-ahead from one: Peek(1) is the token that
	// is about to be read, and Peek(0) is the one just before it. Nearly every
	// caller reports a problem with the token it is looking at but has not yet
	// consumed, so Peek(1) is the one to name. This used to be Peek(0), which
	// named the token before the offending one; that is usually on the same
	// line and so went unnoticed, but when the offending token is the first on
	// its line -- which is true of most statements -- the error was reported
	// against the end of the *previous* line. See docs/issues/REPL-1.md.
	//
	// At the very end of the input there is no next token, and Peek(1) answers
	// with the empty EndOfTokens marker, whose location is 0:0. Falling back to
	// the last real token there keeps "unexpected end of input" style messages
	// pointing at the end of the source rather than at line zero.
	p := c.t.Peek(1)
	if p.Is(tokenizer.EndOfTokens) {
		p = c.t.Peek(0)
	}

	token := ""

	if len(args) > 0 {
		token = data.String(args[0])
	}

	e := errors.New(err).Context(token)

	if c.activePackageName != "" {
		e = e.In(c.activePackageName)
	} else if c.sourceFile != "" {
		e = e.In(c.sourceFile)
	}

	// Get the line and column info from the
	// current token's location info.
	line, col := p.Location()

	return e.At(line+c.lineNumberOffset, col)
}
