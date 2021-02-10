package compiler

import (
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// NewError generates a new compiler error.
func (c *Compiler) NewError(err error, args ...interface{}) *errors.EgoError {
	p := c.t.TokenP
	if p < 0 {
		p = 0
	}

	if p >= len(c.t.Tokens) {
		p = len(c.t.Tokens) - 1
	}

	token := ""

	if len(args) > 0 {
		token = util.GetString(args[0])
	}

	e := errors.New(err).WithContext(token)

	if c.PackageName != "" {
		e = e.In(c.PackageName)
	}

	if p >= 0 {
		e = e.In(c.PackageName).At(c.t.Line[p], c.t.Pos[p])
	}

	return e
}
