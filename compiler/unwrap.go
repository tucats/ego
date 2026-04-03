package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileUnwrap attempts to compile a type-assertion expression of the form
//
//	x.(TypeName)
//
// The "." has already been consumed by the caller (compileDotReference). This
// function tries to match the "(TypeName)" pattern. If it succeeds, an UnWrap
// instruction is emitted that asserts the interface value on the stack holds a
// value of the named type and extracts it.
//
// When the unwrap appears on the left-hand side of a two-variable assignment
// (e.g. "v, ok := x.(T)"), a stack marker and Swap are emitted first so the
// runtime can push both the extracted value and the boolean "ok" flag in the
// right order.
//
// If the pattern does not match (i.e. it is a normal member access, not a type
// assertion), the token position is restored and ErrInvalidUnwrap is returned
// so the caller can fall through to normal member-access handling.
func (c *Compiler) compileUnwrap() error {
	position := c.t.Mark()

	if c.t.IsNext(tokenizer.StartOfListToken) {
		typeName := c.t.Next()
		if typeName.IsIdentifier() {
			if c.t.IsNext(tokenizer.EndOfListToken) {
				if c.flags.inAssignment && c.flags.multipleTargets {
					c.b.Emit(bytecode.Push, bytecode.NewStackMarker("let"))
					c.b.Emit(bytecode.Swap)
				}

				c.flags.hasUnwrap = true
				
				c.b.Emit(bytecode.UnWrap, typeName)

				return nil
			}
		}
	}

	c.t.Set(position)

	return errors.ErrInvalidUnwrap
}
