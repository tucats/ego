package compiler

import (
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/packages"
	"github.com/tucats/ego/tokenizer"
)

// compilePackage compiles a package statement.
func (c *Compiler) compilePackage() error {
	if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
		return c.compileError(errors.ErrMissingPackageName)
	}

	name := c.t.Next()
	if !name.IsIdentifier() {
		return c.compileError(errors.ErrInvalidPackageName, name)
	}

	name = c.normalizeToken(name)

	// Special case -- if this is the "main" package, we have no work to do.
	if name.Spelling() == defs.Main {
		c.flags.mainSeen = true

		return nil
	}

	// If we've already seen a package with this name, we can't redefine it.
	// This is a common error in Go, but we'll check for it here to be more
	// explicit about our error handling.
	if (c.activePackageName != "") && (c.activePackageName != name.Spelling()) {
		return c.compileError(errors.ErrPackageRedefinition)
	}

	c.activePackageName = name.Spelling()

	// We also have to tell the compiler to consider all the builtin symbols
	// from this package to be seen, so they can be referenced within the
	// package without error. If this is the first time we've seen a package
	// statement, then it may not be in the cache yet so skip the defines
	// as yet since there won't be any.
	pkg := packages.Get(name.Spelling())
	if pkg != nil {
		for _, key := range pkg.Keys() {
			c.DefineGlobalSymbol(key)
		}
	}

	return nil
}
