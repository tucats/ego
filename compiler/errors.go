package compiler

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
)

// compileError generates a new compiler compileError.
func (c *Compiler) compileError(err error, args ...any) *errors.Error {
	if c == nil || c.t == nil {
		return errors.New(err)
	}

	p := c.t.Peek(0)
	token := ""

	if len(args) > 0 {
		token = data.String(args[0])
	}

	e := errors.New(err).Context(token)

	if c.activePackageName != "" {
		e.In(c.activePackageName)
	} else if c.sourceFile != "" {
		e.In(c.sourceFile)
	}

	// Get the line and column info from the
	// current token's location info.
	line, col := p.Location()

	return e.At(line+c.lineNumberOffset, col)
}
