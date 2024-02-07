package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compilePackage compiles a package statement.
func (c *Compiler) compilePackage() error {
	if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
		return c.error(errors.ErrMissingPackageName)
	}

	name := c.t.Next()
	if !name.IsIdentifier() {
		return c.error(errors.ErrInvalidPackageName, name)
	}

	name = c.normalizeToken(name)

	if (c.activePackageName != "") && (c.activePackageName != name.Spelling()) {
		return c.error(errors.ErrPackageRedefinition)
	}

	c.activePackageName = name.Spelling()

	c.b.Emit(bytecode.PushPackage, name)

	if name.Spelling() == "main" {
		c.flags.mainSeen = true
	}

	return nil
}
