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
// In every OTHER position -- a bare "v := x.(T)" assignment statement, or
// (BUG-71) any other place a value is expected at all, such as a
// sub-expression, an argument, an operator operand, or an immediately-called
// result -- an IfError instruction is emitted right after UnWrap, so the
// trailing success boolean unwrapByteCode always pushes is consumed on the
// spot: false becomes a catchable ErrTypeMismatch, true is silently dropped,
// and only the asserted value is left on the stack. This matches Go, where
// the single-value form of a type assertion is an ordinary primary
// expression usable anywhere a value is expected -- "x.(int) + 1",
// "f(x.(int))", and "x.(func())()" all compile. Doing the cleanup here,
// immediately, rather than relying on a later, position-specific pass (the
// old approach only handled the direct right-hand side of a "v := x.(T)"
// assignment *statement*, in assignment.go) is what makes every position
// work uniformly.
//
// This IfError is skipped in exactly two cases, both of which push a
// different stack shape than unwrapByteCode's ordinary (value, bool):
//   - the two-value comma-ok form above, where the bool must survive to be
//     stored into the second target;
//   - "x.(type)", the type-switch marker (see form 1 below), which pushes
//     (type, value) instead and is post-processed separately by switch.go.
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
//     name. This form can never be "x.(type)" -- form 1 always intercepts
//     that single-token spelling first -- so its cleanup never needs the
//     type-switch exclusion form 1 does.
func (c *Compiler) compileUnwrap() error {
	position := c.t.Mark()

	if c.t.IsNext(tokenizer.StartOfListToken) {
		typeName := c.t.Next()
		if typeName.IsIdentifier() {
			if c.t.IsNext(tokenizer.EndOfListToken) {
				isTwoValueForm := c.flags.inAssignment && c.flags.multipleTargets

				if isTwoValueForm {
					c.b.Emit(bytecode.Push, bytecode.NewStackMarker("let"))
					c.b.Emit(bytecode.Swap)
				}

				c.flags.hasUnwrap = true

				c.b.Emit(bytecode.UnWrap, typeName)

				if !isTwoValueForm && !typeName.Is(tokenizer.TypeToken) {
					c.b.Emit(bytecode.IfError, errors.ErrTypeMismatch)
				}

				return nil
			}
		}

		// Not a single identifier immediately followed by ")" -- rewind to
		// just after the "(" (which the IsNext() above already consumed)
		// and retry as a compound type specification.
		c.t.Set(position)
		c.t.Advance(1)

		typeSpec, err := c.parseType("", true)
		if err == nil && typeSpec != nil && typeSpec != data.UndefinedType {
			if c.t.IsNext(tokenizer.EndOfListToken) {
				isTwoValueForm := c.flags.inAssignment && c.flags.multipleTargets

				if isTwoValueForm {
					c.b.Emit(bytecode.Push, bytecode.NewStackMarker("let"))
					c.b.Emit(bytecode.Swap)
				}

				c.flags.hasUnwrap = true

				c.b.Emit(bytecode.UnWrap, typeSpec)

				if !isTwoValueForm {
					c.b.Emit(bytecode.IfError, errors.ErrTypeMismatch)
				}

				return nil
			}
		}

		// "chan T" (e.g. x.(chan string)) is never ambiguous with a normal
		// member access -- unlike every other parseType failure here, which
		// falls through below to be retried as "x.member" on the assumption
		// that the parenthesized text just isn't a type spec at all,
		// propagate this one immediately so the caller sees the clear
		// "channels do not have an element type" error instead of the
		// confusing generic "invalid identifier" that a discarded "string"
		// token would otherwise produce (BUG-72).
		if errors.Equals(err, errors.ErrChannelElementType) {
			return err
		}
	}

	c.t.Set(position)

	return errors.ErrInvalidUnwrap
}
