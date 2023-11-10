package compiler

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
)

// error generates a new compiler error.
func (c *Compiler) error(err error, args ...interface{}) *errors.Error {
	if c == nil || c.t == nil {
		return errors.NewError(err)
	}

	p := c.t.TokenP
	if p < 0 {
		p = 0
	}

	if p >= len(c.t.Tokens) {
		p = len(c.t.Tokens) - 1
	}

	token := ""

	if len(args) > 0 {
		token = data.String(args[0])
	}

	e := errors.NewError(err).Context(token)

	if c.activePackageName != "" {
		e = e.In(c.activePackageName)
	} else if c.sourceFile != "" {
		e = e.In(c.sourceFile)
	}

	// Get the context info if possible.
	if p >= 0 && p < len(c.t.Line) && p < len(c.t.Pos) {
		e = e.At(c.t.Line[p], c.t.Pos[p])
	}

	return e
}
