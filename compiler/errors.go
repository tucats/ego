package compiler

import (
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
)

// newError generates a new compiler error.
func (c *Compiler) newError(err error, args ...interface{}) *errors.EgoErrorMsg {
	p := c.t.TokenP
	if p < 0 {
		p = 0
	}

	if p >= len(c.t.Tokens) {
		p = len(c.t.Tokens) - 1
	}

	token := ""

	if len(args) > 0 {
		token = datatypes.String(args[0])
	}

	e := errors.EgoError(err).Context(token)

	if c.activePackageName != "" {
		e = e.In(c.activePackageName)
	} else if c.sourceFile != "" {
		e = e.In(c.sourceFile)
	}

	// Get the context info if possible.
	if p >= 0 && p < len(c.t.Line) && p < len(c.t.Pos) {
		e = e.In(c.activePackageName).At(c.t.Line[p], c.t.Pos[p])
	}

	return e
}
