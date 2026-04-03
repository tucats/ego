package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// reference parses a primary expression atom followed by zero or more
// "suffix" operations that dereference it. The supported suffixes are:
//
//   - .member         — struct/map field access  (dot notation)
//   - [index]         — array or map index, or a slice [start:end]
//   - (args)          — method/function call on the result
//   - {field: value}  — struct initialiser attached to the atom
//
// Each suffix is compiled into bytecode instructions that the runtime will
// execute in sequence, producing the final value on the evaluation stack.
func (c *Compiler) reference() error {
	// Parse the base atom (literal, identifier, parenthesised expression, etc.)
	if err := c.expressionAtom(); err != nil {
		return err
	}

	parsing := true
	// Check for suffix operations in a loop so that chains like
	// a.b[i].c(args) are handled naturally.
	for parsing && !c.t.AtEnd() {
		op := c.t.Peek(1)

		switch {
		// "{" after an expression can mean a struct initialiser, e.g. MyType{x:1}.
		// This is forbidden inside a switch conditional to avoid ambiguity with the
		// switch block's own "{".
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

// compileDotReference compiles the right-hand side of a "." member access. The
// "." token has already been consumed by the caller. Two distinct patterns are
// handled here:
//
//  1. Type unwrap:  x.(TypeName) — asserts that interface value x holds a value
//     of type TypeName and extracts it. Compiled by compileUnwrap().
//
//  2. Normal member access: x.field — looks up the field named "field" inside
//     the struct, map, or package that is currently on the top of the stack.
//     This emits a Member instruction. When "(" follows (a method call), a
//     SetThis instruction is emitted first so the runtime knows the receiver.
//
//  3. Package type initialiser: pkg.Type{field:val} — when a member access is
//     immediately followed by a struct initialiser block, the generated code
//     creates a new struct of the referenced package type.
func (c *Compiler) compileDotReference() error {
	c.t.Advance(1)

	// Try to parse a type-assertion unwrap (x.(T)) first. If it succeeds,
	// the unwrap code has already been emitted and we are done.
	if err := c.compileUnwrap(); err == nil {
		return nil
	}

	// Not an unwrap — the thing after the dot must be a valid identifier.
	lastName := c.t.NextText()
	if !tokenizer.IsSymbol(lastName) {
		return c.compileError(errors.ErrInvalidIdentifier)
	}

	lastName = c.normalize(lastName)

	// If the next token is "(" this is a method call. Emit SetThis so that
	// the runtime stores the receiver value in the implicit "this" variable
	// before transferring control to the method.
	if c.t.Peek(1).Is(tokenizer.StartOfListToken) {
		c.b.Emit(bytecode.SetThis)
	}

	// Emit the Member instruction to dereference the field.
	c.b.Emit(bytecode.Member, lastName)

	// Special case: "pkg.Type{}" is an empty struct initialisation.
	if c.t.IsNext(tokenizer.EmptyInitializerToken) {
		c.b.Emit(bytecode.Load, "$new")
		c.b.Emit(bytecode.Swap)
		c.b.Emit(bytecode.Call, 1)
	} else {
		// "pkg.Type{field:val}" is a struct initialisation with field values.
		// We need to push a marker before the type value so the Struct bytecode
		// can find the boundary on the stack.
		if c.t.Peek(1).Is(tokenizer.DataBeginToken) && c.t.Peek(2).IsIdentifier() && c.t.Peek(3).Is(tokenizer.ColonToken) {
			c.b.Emit(bytecode.Push, bytecode.NewStackMarker("struct-init"))
			c.b.Emit(bytecode.Swap)
			c.b.Emit(bytecode.Push, data.TypeMDKey)

			if err := c.parseStruct(false); err != nil {
				return err
			}

			// The extra TypeMDKey pair we pushed is not counted in the Struct
			// instruction's operand yet — increment it to include that pair.
			i := c.b.Opcodes()
			ix := i[len(i)-1]
			ix.Operand = data.IntOrZero(ix.Operand) + 1
			i[len(i)-1] = ix

			return nil
		}
	}

	return nil
}

// compileArrayIndex compiles an array index or slice expression. The "[" token
// has already been consumed by the caller. Two forms are supported:
//
//  1. Simple index: a[i] — emits a LoadIndex instruction that pops the index
//     and the array/map from the stack and pushes the element at that index.
//
//  2. Slice: a[start:end] or a[:end] or a[start:] — the start and end bounds
//     are compiled as expressions. A missing start defaults to 0; a missing end
//     uses len(a). The runtime LoadSlice instruction creates a new sub-slice.
func (c *Compiler) compileArrayIndex() error {
	c.t.Advance(1)

	t := c.t.Peek(1)
	if t.Is(tokenizer.ColonToken) {
		// "[:" means start from index 0 — push a literal 0 as the lower bound.
		c.b.Emit(bytecode.Push, 0)
	} else {
		if err := c.conditional(); err != nil {
			return err
		}
	}

	// A ":" after the first expression means this is a slice rather than a
	// simple index.
	if c.t.IsNext(tokenizer.ColonToken) {
		if c.t.Peek(1).Is(tokenizer.EndOfArrayToken) {
			// "a[start:]" — use len(a) as the upper bound.
			c.b.Emit(bytecode.Load, "len")
			c.b.Emit(bytecode.ReadStack, -2) // re-read the array from the stack
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
