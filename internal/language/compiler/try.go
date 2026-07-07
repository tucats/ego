package compiler

import (
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// compileTry compiles the try statement which allows the program to catch error
// conditions instead of stopping execution on an error.
func (c *Compiler) compileTry() error {
	// Generate start of a try block.
	b1 := c.b.Mark()
	tryMarker := bytecode.NewStackMarker("try")

	c.b.Emit(bytecode.Try, 0)
	c.b.Emit(bytecode.Push, tryMarker)

	// Statement to try. NOT eligible for PERFORMANCE.md Finding 8 scope
	// elision: if an error occurs partway through the try body, control
	// jumps directly to the catch handler below WITHOUT running this
	// block's own normal-exit PopScope, leaving its scope deliberately
	// still open. The catch clause below stores its error variable into
	// that still-open scope, and an explicit extra PopScope (see below)
	// closes it once the catch body finishes. Eliding this block's own
	// scope would remove the very table that mechanism depends on.
	if err := c.compileRequiredBlock(false, false); err != nil {
		return err
	}

	c.b.Emit(bytecode.DropToMarker, tryMarker)
	b2 := c.b.Mark()

	// The catch block is optional. If not found, patch up the try destination to here
	// and generate a pop of the try stack.
	if !c.t.IsNext(tokenizer.CatchToken) {
		_ = c.b.SetAddressHere(b1)
		c.b.Emit(bytecode.TryPop)

		return nil
	}

	// Need to generate a branch around the catch block for success cases.
	c.b.Emit(bytecode.Branch, 0)
	_ = c.b.SetAddressHere(b1)

	// Is there a named variable that will hold the error?

	if c.t.IsNext(tokenizer.StartOfListToken) {
		errName := c.t.Next()
		if !errName.IsIdentifier() {
			return c.compileError(errors.ErrInvalidSymbolName)
		}

		if !c.t.IsNext(tokenizer.EndOfListToken) {
			return c.compileError(errors.ErrMissingParenthesis)
		}

		c.b.Emit(bytecode.Load, defs.ErrorVariable)
		c.b.Emit(bytecode.StoreAlways, errName)
		c.DefineSymbol(errName.Spelling())
	}

	// The catch body has its own, separate scope layer (this call), nested
	// inside the try body's still-open scope (see above). Eliding THIS
	// layer, when the catch body itself declares nothing, is safe: the
	// error variable "e" was already stored into the try body's scope,
	// above, before this scope would even be pushed, so it is unaffected
	// either way -- and the extra PopScope below always closes exactly the
	// try body's scope, regardless of whether this call pushed one of its
	// own in between.
	if err := c.compileRequiredBlock(false, true); err != nil {
		return err
	}
	// Need extra PopScope because we're still running in the scope of the try{} block
	c.b.Emit(bytecode.PopScope)

	// This marks the end of the try/catch
	_ = c.b.SetAddressHere(b2)
	c.b.Emit(bytecode.TryPop)

	return nil
}
