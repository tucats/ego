package compiler

import (
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/tokenizer"
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
//
// Two forms of assertion target are recognized, tried in order:
//
//  1. A single IDENTIFIER token immediately followed by ")" -- this covers a
//     plain type name (a user-defined type, a built-in primitive like "int",
//     "any", or the "type" keyword used by the "switch v := x.(type)" form).
//     The raw token is passed as the UnWrap operand and resolved by NAME at
//     runtime (see unwrapByteCode in bytecode/types.go), exactly as before.
//
//  2. Anything else is retried as a general type specification using the
//     same parser type-cast expressions (T(value)) use (parseType). This is
//     what makes a compound assertion target -- a pointer, slice, map,
//     struct, interface literal, or function type such as "func() int" --
//     compile. Previously only form 1 was attempted, so "x.(func() int)"
//     failed to compile with "invalid identifier" because "func" is a
//     keyword token, not an IDENTIFIER (BUG-60). Since a compound type has
//     no single name to look up by string, the *data.Type resolved here at
//     compile time is passed as the UnWrap operand directly, instead of a
//     name.
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

		// Not a single identifier immediately followed by ")" -- rewind to
		// just after the "(" (which the IsNext() above already consumed)
		// and retry as a compound type specification.
		c.t.Set(position)
		c.t.Advance(1)

		if typeSpec, err := c.parseType("", true); err == nil && typeSpec != nil && typeSpec != data.UndefinedType {
			if c.t.IsNext(tokenizer.EndOfListToken) {
				if c.flags.inAssignment && c.flags.multipleTargets {
					c.b.Emit(bytecode.Push, bytecode.NewStackMarker("let"))
					c.b.Emit(bytecode.Swap)
				}

				c.flags.hasUnwrap = true

				c.b.Emit(bytecode.UnWrap, typeSpec)

				return nil
			}
		}
	}

	c.t.Set(position)

	return errors.ErrInvalidUnwrap
}
