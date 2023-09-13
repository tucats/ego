package compiler

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

func (c *Compiler) expressionAtom() error {
	t := c.t.Peek(1)

	// Is it a short-form try/catch?
	if t == tokenizer.OptionalToken {
		c.t.Advance(1)

		return c.optional()
	}

	// Is it a binary constant? If so, convert to decimal.
	text := t.Spelling()
	if strings.HasPrefix(strings.ToLower(text), "0b") {
		binaryValue := 0
		fmt.Sscanf(text[2:], "%b", &binaryValue)
		t = tokenizer.NewIntegerToken(strconv.Itoa(binaryValue))
	}

	// Is it a hexadecimal constant? If so, convert to decimal.
	if strings.HasPrefix(strings.ToLower(text), "0x") {
		hexValue := 0
		fmt.Sscanf(strings.ToLower(text[2:]), "%x", &hexValue)
		t = tokenizer.NewIntegerToken(strconv.Itoa(hexValue))
	}

	// Is it an octal constant? If so, convert to decimal.
	if strings.HasPrefix(strings.ToLower(text), "0o") {
		octalValue := 0
		fmt.Sscanf(strings.ToLower(text[2:]), "%o", &octalValue)
		t = tokenizer.NewIntegerToken(strconv.Itoa(octalValue))
	}

	// Is this the "nil" constant?
	if t == tokenizer.NilToken {
		c.t.Advance(1)
		c.b.Emit(bytecode.Push, nil)

		return nil
	}

	// Is an empty struct?
	if t == tokenizer.EmptyInitializerToken {
		c.t.Advance(1)
		c.b.Emit(bytecode.Push, data.NewStruct(data.StructType).SetStatic(false))

		return nil
	}

	// Is this a function definition?
	if t == tokenizer.FuncToken && c.t.Peek(2) == tokenizer.StartOfListToken {
		c.t.Advance(1)

		return c.compileFunctionDefinition(true)
	}

	// Is this address-of?
	if t == tokenizer.AddressToken {
		c.t.Advance(1)

		// If it's address of a symbol, short-circuit that. We do not allow
		// readonly symbols to be addressable.
		if c.t.Peek(1).IsIdentifier() {
			name := c.t.Next()
			if strings.HasPrefix(name.Spelling(), defs.ReadonlyVariablePrefix) {
				return c.error(errors.ErrReadOnlyAddressable, name)
			}

			c.b.Emit(bytecode.AddressOf, name)
		} else {
			// Address of an expression requires creating a temp symbol
			if err := c.expressionAtom(); err != nil {
				return err
			}

			tempName := data.GenerateName()

			c.b.Emit(bytecode.StoreAlways, tempName)
			c.b.Emit(bytecode.AddressOf, tempName)
		}

		return nil
	}

	// Is this dereference?
	if t == tokenizer.PointerToken {
		c.t.Advance(1)

		// If it's dereference of a symbol, short-circuit that
		if c.t.Peek(1).IsIdentifier() {
			name := c.t.Next()
			c.b.Emit(bytecode.DeRef, name)
		} else {
			// Dereference of an expression requires creating a temp symbol
			if err := c.expressionAtom(); err != nil {
				return err
			}

			tempName := data.GenerateName()

			c.b.Emit(bytecode.StoreAlways, tempName)
			c.b.Emit(bytecode.DeRef, tempName)
		}

		return nil
	}

	// Is this a parenthesis expression?
	if t == tokenizer.StartOfListToken {
		c.t.Advance(1)

		if err := c.conditional(); err != nil {
			return err
		}

		if c.t.Next() != tokenizer.EndOfListToken {
			return c.error(errors.ErrMissingParenthesis)
		}

		return nil
	}

	// Is this an array constant?
	if t == tokenizer.StartOfArrayToken && c.t.Peek(2) != tokenizer.EndOfArrayToken {
		return c.parseArray()
	}

	// Is it a map constant?
	if t == tokenizer.DataBeginToken {
		return c.parseStruct()
	}

	if t == tokenizer.StructToken && c.t.Peek(2) == tokenizer.DataBeginToken {
		c.t.Advance(1)

		return c.parseStruct()
	}

	// If the token is a number, convert it
	if t.IsClass(tokenizer.IntegerTokenClass) {
		if i, err := strconv.ParseInt(text, 10, 32); err == nil {
			c.t.Advance(1)
			c.b.Emit(bytecode.Push, int(i))

			return nil
		}

		if i, err := strconv.ParseInt(text, 10, 64); err == nil {
			c.t.Advance(1)
			c.b.Emit(bytecode.Push, i)

			return nil
		}
	}

	if t.IsClass(tokenizer.FloatTokenClass) {
		if i, err := strconv.ParseFloat(text, 64); err == nil {
			c.t.Advance(1)
			c.b.Emit(bytecode.Push, i)

			return nil
		}
	}

	if t.IsClass(tokenizer.BooleanTokenClass) {
		if text == defs.True || text == defs.False {
			c.t.Advance(1)
			c.b.Emit(bytecode.Push, (text == defs.True))

			return nil
		}
	}

	if t.IsValue() {
		c.t.Advance(1)
		c.b.Emit(bytecode.Push, t)

		return nil
	}

	// Is it a type cast?
	if c.t.Peek(2) == tokenizer.StartOfListToken {
		mark := c.t.Mark()

		if typeSpec, err := c.parseType("", true); err == nil {
			if c.t.IsNext(tokenizer.StartOfListToken) { // Skip the parentheses
				b, err := c.Expression()
				if err == nil {
					for c.t.IsNext(tokenizer.CommaToken) {
						b2, e2 := c.Expression()
						if e2 == nil {
							b.Append(b2)
						} else {
							err = e2

							break
						}
					}
				}

				if err == nil && c.t.Peek(1) == tokenizer.EndOfListToken {
					c.t.Next()
					c.b.Emit(bytecode.Push, typeSpec)
					c.b.Append(b)
					c.b.Emit(bytecode.Call, 1)

					return err
				}
			}
		}

		c.t.Set(mark)
	}

	// Watch out for function calls here.
	isPanic := c.flags.extensionsEnabled && (c.t.Peek(1) == tokenizer.PanicToken)
	if !isPanic && c.t.Peek(2) != tokenizer.StartOfListToken {
		marker := c.t.Mark()

		if typeSpec, err := c.parseType("", true); err == nil {
			// Is there an initial value for the type?
			if c.t.Peek(1) == tokenizer.DataBeginToken {
				err = c.compileInitializer(typeSpec)

				return err
			}

			if c.t.IsNext(tokenizer.EmptyInitializerToken) {
				c.b.Emit(bytecode.Load, "$new")
				c.b.Emit(bytecode.Push, typeSpec)
				c.b.Emit(bytecode.Call, 1)
			} else {
				c.b.Emit(bytecode.Push, typeSpec)
			}

			return nil
		}

		c.t.Set(marker)
	}

	// Is it just a symbol needing a load?
	if t.IsIdentifier() {
		// Check for auto-increment or decrement
		autoMode := bytecode.NoOperation

		if c.t.Peek(2) == tokenizer.IncrementToken {
			autoMode = bytecode.Add
		}

		if c.t.Peek(2) == tokenizer.DecrementToken {
			autoMode = bytecode.Sub
		}

		// If language extensions are supported and this is an auto-increment
		// or decrement operation, do it now. The modification is applied after
		// the value is read; i.e. the atom is the pre-modified value.
		if c.flags.extensionsEnabled && (autoMode != bytecode.NoOperation) {
			c.b.Emit(bytecode.Load, t)
			c.b.Emit(bytecode.Dup)
			c.b.Emit(bytecode.Push, 1)
			c.b.Emit(autoMode)
			c.b.Emit(bytecode.Store, t)
			c.t.Advance(2)
		} else {
			c.b.Emit(bytecode.Load, t)
			c.t.Advance(1)
		}

		return nil
	}

	// Not something we know what to do with...
	return c.error(errors.ErrUnexpectedToken, t)
}

func (c *Compiler) parseArray() error {
	var (
		err, e2        error
		listTerminator = tokenizer.EmptyToken
	)

	// Lets see if this is a type name. Remember where
	// we came from, and back up over the previous "["
	// already parsed in the expression atom.
	marker := c.t.Mark()

	kind, err := c.parseTypeSpec()
	if err != nil {
		return err
	}

	if !kind.IsUndefined() {
		if !kind.IsArray() {
			return c.error(errors.ErrInvalidTypeName)
		}

		// Is it an empty declaration, such as []int{} ?
		if c.t.IsNext(tokenizer.EmptyInitializerToken) {
			c.b.Emit(bytecode.Array, 0, kind)

			return nil
		}

		// There better be at least the start of an initialization block then.
		if !c.t.IsNext(tokenizer.DataBeginToken) {
			return c.error(errors.ErrMissingBlock)
		}

		listTerminator = tokenizer.DataEndToken
	} else {
		c.t.Set(marker)

		if c.t.Peek(1) == tokenizer.StartOfListToken {
			listTerminator = tokenizer.EndOfListToken
		} else {
			if c.t.Peek(1) == tokenizer.StartOfArrayToken {
				listTerminator = tokenizer.EndOfArrayToken
			}
		}

		c.t.Advance(1)

		// Let's experimentally see if this is a range constant expression. This can be
		// of the form [start:end] which creates an array of integers between the start
		// and end values (inclusive). It can also be of the form [:end] which assumes
		// a start number of 1.
		t1 := 1

		if c.t.Peek(1) == tokenizer.ColonToken {
			err = nil

			c.t.Advance(-1)
		} else {
			t1, e2 = strconv.Atoi(c.t.PeekText(1))
			if e2 != nil {
				err = errors.NewError(e2)
			}
		}

		if err == nil {
			if c.t.Peek(2) == tokenizer.ColonToken {
				t2, err := strconv.Atoi(c.t.PeekText(3))
				if err == nil {
					c.t.Advance(3)

					count := t2 - t1 + 1
					if count < 0 {
						count = (-count) + 2

						for n := t1; n >= t2; n = n - 1 {
							c.b.Emit(bytecode.Push, n)
						}
					} else {
						for n := t1; n <= t2; n = n + 1 {
							c.b.Emit(bytecode.Push, n)
						}
					}

					c.b.Emit(bytecode.Array, count)

					if !c.t.IsNext(tokenizer.EndOfArrayToken) {
						return c.error(errors.ErrInvalidRange)
					}

					return nil
				}
			}
		}
	}

	if listTerminator == tokenizer.EmptyToken {
		return nil
	}

	count := 0

	for c.t.Peek(1) != listTerminator {
		if err := c.conditional(); err != nil {
			return err
		}

		// If this is an array of a specific type, check to see
		// if the previous value was a constant. If it wasn't, or
		// was of the wrong type, emit a coerce...
		if !kind.IsUndefined() {
			if c.b.NeedsCoerce(kind) {
				c.b.Emit(bytecode.Coerce, kind)
			}
		}

		count = count + 1

		if c.t.AtEnd() {
			break
		}

		if c.t.Peek(1) == listTerminator {
			break
		}

		if c.t.Peek(1) != tokenizer.CommaToken {
			return c.error(errors.ErrInvalidList)
		}

		c.t.Advance(1)
	}

	if !kind.IsUndefined() {
		c.b.Emit(bytecode.Array, count, kind)
	} else {
		c.b.Emit(bytecode.Array, count)
	}

	c.t.Advance(1)

	return nil
}

func (c *Compiler) parseStruct() error {
	var listTerminator = tokenizer.DataEndToken

	var err error

	c.t.Advance(1)

	count := 0

	for c.t.Peek(1) != listTerminator {
		// First element: name
		name := c.t.Next()
		if !name.IsString() && !name.IsIdentifier() {
			return c.error(errors.ErrInvalidSymbolName, name)
		}

		name = c.normalizeToken(name)

		// Second element: colon
		if c.t.Next() != tokenizer.ColonToken {
			return c.error(errors.ErrMissingColon)
		}

		// Third element: value, which is emitted.
		if err := c.conditional(); err != nil {
			return err
		}

		// Now write the name as a string.
		c.b.Emit(bytecode.Push, name)

		count = count + 1

		if c.t.AtEnd() {
			break
		}

		if c.t.Peek(1) == listTerminator {
			break
		}

		if c.t.Peek(1) != tokenizer.CommaToken {
			return c.error(errors.ErrInvalidList)
		}

		c.t.Advance(1)
	}

	c.b.Emit(bytecode.Struct, count)
	c.t.Advance(1)

	if err != nil {
		err = errors.NewError(err)
	}

	return err
}

// Handle the ? optional operation. This precedes an expression
// element. If the element causes an error then a default value is
// provided following a ":" operator. This only works for specific
// errors such as a nil object reference, invalid type, unknown
// structure member, or divide-by-zero.
//
// This is only supported when extensions are enabled.
func (c *Compiler) optional() error {
	if !c.flags.extensionsEnabled {
		return c.error(errors.ErrUnexpectedToken).Context("?")
	}

	catch := c.b.Mark()
	c.b.Emit(bytecode.Try)

	// What errors do we permit here?
	c.b.Emit(bytecode.WillCatch, bytecode.OptionalCatchSet)

	if err := c.unary(); err != nil {
		return err
	}

	toEnd := c.b.Mark()
	c.b.Emit(bytecode.Branch)
	_ = c.b.SetAddressHere(catch)

	if !c.t.IsNext(tokenizer.ColonToken) {
		return c.error(errors.ErrMissingCatch)
	}

	if err := c.unary(); err != nil {
		return err
	}

	_ = c.b.SetAddressHere(toEnd)
	c.b.Emit(bytecode.TryPop)

	return nil
}
