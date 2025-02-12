package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/defs"
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

	// Special case -- if this is the "main" package, we have no work to do.
	if name.Spelling() == defs.Main {
		return nil
	}

	// If we've already seen a package with this name, we can't redefine it.
	// This is a common error in Go, but we'll check for it here to be more
	// explicit about our error handling.
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
