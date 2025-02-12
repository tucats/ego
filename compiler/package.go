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

	// Special case -- if this is the "main" package, we have no work to do.
	if name.Spelling() == "main" {
		return nil
	}

	// If we've already seen a package with this name, we can't redefine it.
	// This is a common error in Go, but we'll check for it here to be more
	// explicit about our error handling.
	//
	// Note: This check is done in the order of the packages being imported,
	// so if we see a package we're not expecting, we know we're redefining
	// something. This is a limitation of our current design. If we want to
	// support multiple packages with the same name, we'll need to change the
	// way we handle imports and redefinitions.
	//
	// TODO: Fix this limitation.
	//
	// Also, note that this check is not done during the actual compilation
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
