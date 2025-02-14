package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// compileConst compiles a constant block.
func (c *Compiler) compileConst() error {
	// Is this a list of constants enclosed in a parenthesis?
	terminator := tokenizer.EmptyToken
	isList := false

	if c.t.IsNext(tokenizer.StartOfListToken) {
		terminator = tokenizer.EndOfListToken
		isList = true
	}

	// Scan over the list (possibly a single item) and compile each
	// constant. These are essentially expressions which are stored
	// away as readonly symbols.
	for terminator.Is(tokenizer.EmptyToken) || !c.t.IsNext(terminator) {
		name := c.t.Next()
		if isList && name.Is(tokenizer.SemicolonToken) {
			if c.t.IsNext(terminator) {
				break
			}

			name = c.t.Next()
		}

		if !name.IsIdentifier() {
			return c.compileError(errors.ErrInvalidSymbolName)
		}

		nameSpelling := c.normalize(name.Spelling())

		if !c.t.IsNext(tokenizer.AssignToken) {
			return c.compileError(errors.ErrMissingEqual)
		}

		vx, err := c.Expression(true)
		if err != nil {
			return err
		}

		// Search to make sure the resulting expression doesn't contain a load statement that
		// isn't for another constant. That would indicate that the expression value itself
		// is not truly constant. We keep a list of all constant values found by this compiler
		// instance.
		for _, i := range vx.Opcodes() {
			if i.Operation == bytecode.Load && !util.InList(data.String(i.Operand), c.constants...) {
				return c.compileError(errors.ErrInvalidConstant)
			}
		}

		// It's a constant expression. Save the constant name in our list for future comparisons, and
		// emit the Constant bytecode which stores the value.
		c.constants = append(c.constants, nameSpelling)

		c.b.Append(vx)
		c.b.Emit(bytecode.Constant, nameSpelling)

		// If this wasn't a list, we're done.
		if terminator.Is(tokenizer.EmptyToken) {
			break
		}
	}

	return nil
}
