package compiler

import (
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
)

// newError generates a new compiler error.
func (c *Compiler) newError(err error, args ...interface{}) *errors.EgoError {
	p := c.t.TokenP
	if p < 0 {
		p = 0
	}

	if p >= len(c.t.Tokens) {
		p = len(c.t.Tokens) - 1
	}

	token := ""

	if len(args) > 0 {
		token = datatypes.GetString(args[0])
	}

	e := errors.New(err).Context(token)

	if c.PackageName != "" {
		e = e.In(c.PackageName)
	}

	// Get the context info if possible.
	if p >= 0 && p < len(c.t.Line) && p < len(c.t.Pos) {
		e = e.In(c.PackageName).At(c.t.Line[p], c.t.Pos[p])
	}

	return e
}
