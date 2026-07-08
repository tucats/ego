package compiler

import (
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
)

// compileThrow compiles the "throw" statement, a language extension similar
// in spirit to try/catch. The statement takes a single expression, which is
// expected to be an error value:
//
//	throw err
//
// If the expression evaluates to a non-nil error, it is raised as a
// trappable runtime error -- exactly as if a genuine runtime error (such as
// division by zero) had occurred -- so it can be caught by an enclosing
// try/catch. If the expression evaluates to nil (or Ego's zero-value
// error), nothing happens and execution continues with the next statement.
// This lets "throw err" stand in for Go's "if err != nil { return err }"
// idiom, collapsed into a single statement, when the intent is purely to
// propagate an error condition rather than to return a specific value.
//
// Like other extensions (try/catch, print, exit, ...), "throw" is only
// recognized when language extensions are enabled; the caller in
// statement.go already checks c.flags.extensionsEnabled before calling this
// function.
func (c *Compiler) compileThrow() error {
	if c.atStatementEnd() {
		return c.compileError(errors.ErrMissingExpression)
	}

	if err := c.emitExpression(); err != nil {
		return err
	}

	// Verify (and, in relaxed/dynamic mode, coerce) the expression's value
	// against the built-in "error" type before throwing it. This reuses the
	// same conformance-check machinery used for a declared "error" function
	// parameter, including the BUG-65 fix that lets a nil value satisfy it.
	c.b.Emit(bytecode.RequiredType, data.ErrorType)
	c.b.Emit(bytecode.Throw, nil)

	return nil
}
