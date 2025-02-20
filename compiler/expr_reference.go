package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// reference parses a structure or array reference.
func (c *Compiler) reference() error {
	// Parse the function call or expression atom
	if err := c.expressionAtom(); err != nil {
		return err
	}

	parsing := true
	// is there a trailing structure or array reference?
	for parsing && !c.t.AtEnd() {
		op := c.t.Peek(1)

		switch {
		// Structure initialization
		case op.Is(tokenizer.DataBeginToken):
			// If this is during switch statement processing, it can't be
			// a structure initialization.
			if c.flags.disallowStructInits {
				return nil
			}

			name := c.t.Peek(2)
			colon := c.t.Peek(3)

			if name.IsIdentifier() && colon.Is(tokenizer.ColonToken) {
				c.b.Emit(bytecode.Push, data.TypeMDKey)

				if err := c.expressionAtom(); err != nil {
					return err
				}

				i := c.b.Opcodes()
				ix := i[len(i)-1]
				ix.Operand = data.IntOrZero(ix.Operand) + 1 // __type
				i[len(i)-1] = ix
			} else {
				parsing = false
			}
		// Function invocation
		case op.Is(tokenizer.StartOfListToken):
			c.t.Advance(1)

			if err := c.functionCall(); err != nil {
				return err
			}

		// Map member reference
		case op.Is(tokenizer.DotToken):
			// Peek ahead. is this a chained call? If so, set the This
			// value
			// Is it a generator for a type?
			// __type and
			err := c.compileDotReference()
			if err != nil {
				return err
			}

		// Array index reference
		case op.Is(tokenizer.StartOfArrayToken):
			err := c.compileArrayIndex()
			if err != nil {
				return err
			}

		// Nothing else, term is complete
		default:
			return nil
		}
	}

	return nil
}

func (c *Compiler) compileDotReference() error {
	c.t.Advance(1)

	// Is it a type unwrap like foo.(int)?
	if err := c.compileUnwrap(); err == nil {
		return nil
	}

	// What are we dereferencing here? It must be a valid identifier.
	lastName := c.t.NextText()
	if !tokenizer.IsSymbol(lastName) {
		return c.compileError(errors.ErrInvalidIdentifier)
	}

	lastName = c.normalize(lastName)

	// If it smells like a method call, make a note of the "this" value.
	if c.t.Peek(1).Is(tokenizer.StartOfListToken) {
		c.b.Emit(bytecode.SetThis)
	}

	// Do the dereference operation.
	c.b.Emit(bytecode.Member, lastName)

	// Is it an initializer for a type from a package (which would have looked just like a structure dereference)?
	if c.t.IsNext(tokenizer.EmptyInitializerToken) {
		c.b.Emit(bytecode.Load, "$new")
		c.b.Emit(bytecode.Swap)
		c.b.Emit(bytecode.Call, 1)
	} else {
		if c.t.Peek(1).Is(tokenizer.DataBeginToken) && c.t.Peek(2).IsIdentifier() && c.t.Peek(3).Is(tokenizer.ColonToken) {
			// The stack already has the type value on the stack at this point. We need to put a marker before it,
			// so generate code that pushes the marker and then swaps the top two items. Then add the type key name
			// that pairs with the actual type value.
			c.b.Emit(bytecode.Push, bytecode.NewStackMarker("struct-init"))
			c.b.Emit(bytecode.Swap)
			c.b.Emit(bytecode.Push, data.TypeMDKey)

			if err := c.parseStruct(false); err != nil {
				return err
			}

			// Update the structure count in the bytecode to include the extra pair we put on the stack
			// already with the TypeMDKey value.
			i := c.b.Opcodes()
			ix := i[len(i)-1]
			ix.Operand = data.IntOrZero(ix.Operand) + 1
			i[len(i)-1] = ix

			return nil
		}
	}

	return nil
}

// Compile an array index reference. The leading "[" has already been consumed.
func (c *Compiler) compileArrayIndex() error {
	c.t.Advance(1)

	t := c.t.Peek(1)
	if t.Is(tokenizer.ColonToken) {
		c.b.Emit(bytecode.Push, 0)
	} else {
		if err := c.conditional(); err != nil {
			return err
		}
	}

	// Could be a range or slice.
	if c.t.IsNext(tokenizer.ColonToken) {
		if c.t.Peek(1).Is(tokenizer.EndOfArrayToken) {
			c.b.Emit(bytecode.Load, "len")
			c.b.Emit(bytecode.ReadStack, -2)
			c.b.Emit(bytecode.Call, 1)
		} else {
			if err := c.conditional(); err != nil {
				return err
			}
		}

		c.b.Emit(bytecode.LoadSlice)

		if !c.t.Next().Is(tokenizer.EndOfArrayToken) {
			return c.compileError(errors.ErrMissingBracket)
		}
	} else {
		if !c.t.Next().Is(tokenizer.EndOfArrayToken) {
			return c.compileError(errors.ErrMissingBracket)
		}

		c.b.Emit(bytecode.LoadIndex)
	}

	return nil
}
