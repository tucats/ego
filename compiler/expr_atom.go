package compiler

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

func (c *Compiler) expressionAtom() *errors.EgoError {
	t := c.t.Peek(1)

	// Is it a short-form try/catch?
	if t == "?" {
		c.t.Advance(1)

		return c.optional()
	}

	// Is it a binary constant? If so, convert to decimal.
	if strings.HasPrefix(strings.ToLower(t), "0b") {
		binaryValue := 0
		fmt.Sscanf(t[2:], "%b", &binaryValue)
		t = strconv.Itoa(binaryValue)
	}

	// Is it a hexadecimal constant? If so, convert to decimal.
	if strings.HasPrefix(strings.ToLower(t), "0x") {
		hexValue := 0
		fmt.Sscanf(strings.ToLower(t[2:]), "%x", &hexValue)
		t = strconv.Itoa(hexValue)
	}

	// Is it an octal constant? If so, convert to decimal.
	if strings.HasPrefix(strings.ToLower(t), "0o") {
		octalValue := 0
		fmt.Sscanf(strings.ToLower(t[2:]), "%o", &octalValue)
		t = strconv.Itoa(octalValue)
	}

	// Is this the make() function?
	if t == "make" && c.t.Peek(2) == "(" {
		return c.makeInvocation()
	}

	// Is this the "nil" constant?
	if t == "nil" {
		c.t.Advance(1)
		c.b.Emit(bytecode.Push, nil)

		return nil
	}

	// Is an empty struct?
	if t == "{}" {
		c.t.Advance(1)
		c.b.Emit(bytecode.Push, datatypes.NewStruct(datatypes.StructType).SetStatic(false))

		return nil
	}

	// Is this a function definition?
	if t == "func" && c.t.Peek(2) == "(" {
		c.t.Advance(1)

		return c.compileFunctionDefinition(true)
	}

	// Is this address-of?
	if t == "&" {
		c.t.Advance(1)

		// If it's address of a symbol, short-circuit that
		if tokenizer.IsSymbol(c.t.Peek(1)) {
			name := c.t.Next()
			c.b.Emit(bytecode.AddressOf, name)
		} else {
			// Address of an expression requires creating a temp symbol
			err := c.expressionAtom()
			if !errors.Nil(err) {
				return err
			}

			tempName := datatypes.GenerateName()

			c.b.Emit(bytecode.StoreAlways, tempName)
			c.b.Emit(bytecode.AddressOf, tempName)
		}

		return nil
	}

	// Is this dereference?
	if t == "*" {
		c.t.Advance(1)

		// If it's dereference of a symbol, short-circuit that
		if tokenizer.IsSymbol(c.t.Peek(1)) {
			name := c.t.Next()
			c.b.Emit(bytecode.DeRef, name)
		} else {
			// Dereference of an expression requires creating a temp symbol
			err := c.expressionAtom()
			if !errors.Nil(err) {
				return err
			}

			tempName := datatypes.GenerateName()

			c.b.Emit(bytecode.StoreAlways, tempName)
			c.b.Emit(bytecode.DeRef, tempName)
		}

		return nil
	}

	// Is this a parenthesis expression?
	if t == "(" {
		c.t.Advance(1)

		err := c.conditional()
		if !errors.Nil(err) {
			return err
		}

		if c.t.Next() != ")" {
			return c.newError(errors.ErrMissingParenthesis)
		}

		return nil
	}

	// Is this an array constant?
	if t == "[" && c.t.Peek(2) != "]" {
		return c.parseArray()
	}

	// Is it a map constant?
	if t == "{" {
		return c.parseStruct()
	}

	if t == "struct" && c.t.Peek(2) == "{" {
		c.t.Advance(1)

		return c.parseStruct()
	}

	// If the token is a number, convert it
	if i, err := strconv.Atoi(t); errors.Nil(err) {
		c.t.Advance(1)
		c.b.Emit(bytecode.Push, i)

		return nil
	}

	if i, err := strconv.ParseFloat(t, 64); errors.Nil(err) {
		c.t.Advance(1)
		c.b.Emit(bytecode.Push, i)

		return nil
	}

	if t == defs.True || t == defs.False {
		c.t.Advance(1)
		c.b.Emit(bytecode.Push, (t == defs.True))

		return nil
	}

	runeValue := t[0:1]
	if runeValue == "\"" {
		c.t.Advance(1)

		s, err := strconv.Unquote(t)

		c.b.Emit(bytecode.Push, s)

		return errors.New(err)
	}

	if runeValue == "`" {
		c.t.Advance(1)

		s, err := c.unLit(t)

		c.b.Emit(bytecode.Push, s)

		return err
	}

	// Is it a type cast?
	if c.t.Peek(2) == "(" {
		mark := c.t.Mark()

		if typeSpec, err := c.parseType(true); err == nil {
			if c.t.IsNext("(") { // Skip the parentheses
				b, err := c.Expression()
				if err == nil {
					for c.t.IsNext(",") {
						b2, e2 := c.Expression()
						if e2 == nil {
							b.Append(b2)
						} else {
							err = e2

							break
						}
					}
				}

				if err == nil && c.t.Peek(1) == ")" {
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

	// Watch out for function calls here...
	if c.t.Peek(2) != "(" {
		marker := c.t.Mark()

		if typeSpec, err := c.parseType(true); err == nil {
			// Is there an initial value for the type?
			if c.t.Peek(1) == "{" {
				err = c.compileInitializer(typeSpec)

				return err
			}

			if c.t.IsNext("{}") {
				c.b.Emit(bytecode.Load, "new")
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
	if tokenizer.IsSymbol(t) {
		// Check for auto-increment or decrement
		autoMode := bytecode.Load

		if c.t.Peek(2) == "++" {
			autoMode = bytecode.Add
		}

		if c.t.Peek(2) == "--" {
			autoMode = bytecode.Sub
		}

		// If language extensions are supported and this is an auto-increment
		// or decrement operation, do it now. The modification is applied after
		// the value is read; i.e. the atom is the pre-modified value.
		if c.extensionsEnabled && (autoMode != bytecode.Load) {
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
	return c.newError(errors.ErrUnexpectedToken, t)
}

func (c *Compiler) parseArray() *errors.EgoError {
	var err *errors.EgoError

	var listTerminator = ""

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
			return c.newError(errors.ErrInvalidTypeName)
		}

		// It could be a cast operation. If so, remember where we are
		// in case we need to back up, and take a look...
		if c.t.IsNext("(") {
			mark := c.t.Mark() - 1

			exp, e2 := c.Expression()
			if e2 == nil && c.t.IsNext(")") {
				c.b.Emit(bytecode.Load, "$cast")
				c.b.Append(exp)
				c.b.Emit(bytecode.Push, kind)
				c.b.Emit(bytecode.Call, 2)

				return nil
			}

			c.t.Set(mark)
		}
		// Is it an empty declaration, such as []int{} ?
		if c.t.IsNext("{}") {
			c.b.Emit(bytecode.Array, 0, kind)

			return nil
		}

		// There better be at least the start of an initialization block then.
		if !c.t.IsNext("{") {
			return c.newError(errors.ErrMissingBlock)
		}

		listTerminator = "}"
	} else {
		c.t.Set(marker)

		if c.t.Peek(1) == "(" {
			listTerminator = ")"
		} else {
			if c.t.Peek(1) == "[" {
				listTerminator = "]"
			}
		}
		c.t.Advance(1)

		// Let's experimentally see if this is a range constant expression. This can be
		// of the form [start:end] which creates an array of integers between the start
		// and end values (inclusive). It can also be of the form [:end] which assumes
		// a start number of 1.

		t1 := 1

		var e2 error

		if c.t.Peek(1) == ":" {
			err = nil

			c.t.Advance(-1)
		} else {
			t1, e2 = strconv.Atoi(c.t.Peek(1))
			if e2 != nil {
				err = errors.New(e2)
			}
		}

		if errors.Nil(err) {
			if c.t.Peek(2) == ":" {
				t2, err := strconv.Atoi(c.t.Peek(3))
				if errors.Nil(err) {
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

					if !c.t.IsNext("]") {
						return c.newError(errors.ErrInvalidRange)
					}

					return nil
				}
			}
		}
	}

	if listTerminator == "" {
		return nil
	}

	count := 0

	for c.t.Peek(1) != listTerminator {
		err := c.conditional()
		if !errors.Nil(err) {
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

		if c.t.Peek(1) != "," {
			return c.newError(errors.ErrInvalidList)
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

func (c *Compiler) parseStruct() *errors.EgoError {
	var listTerminator = "}"

	var err error

	c.t.Advance(1)

	count := 0

	for c.t.Peek(1) != listTerminator {
		// First element: name
		name := c.t.Next()
		if len(name) > 2 && name[0:1] == "\"" {
			name, err = strconv.Unquote(name)
			if !errors.Nil(err) {
				return errors.New(err)
			}
		} else {
			if !tokenizer.IsSymbol(name) {
				return c.newError(errors.ErrInvalidSymbolName, name)
			}
		}

		name = c.normalize(name)

		// Second element: colon
		if c.t.Next() != ":" {
			return c.newError(errors.ErrMissingColon)
		}

		// Third element: value, which is emitted.
		err := c.conditional()
		if !errors.Nil(err) {
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

		if c.t.Peek(1) != "," {
			return c.newError(errors.ErrInvalidList)
		}

		c.t.Advance(1)
	}

	c.b.Emit(bytecode.Struct, count)
	c.t.Advance(1)

	return errors.New(err)
}

func (c *Compiler) unLit(s string) (string, *errors.EgoError) {
	if quote := s[0:1]; s[len(s)-1:] != quote {
		return s[1:], c.newError(errors.ErrBlockQuote, quote)
	}

	return s[1 : len(s)-1], nil
}

// Handle the ? optional operation. This precedes an expression
// element. If the element causes an error then a default value is
// provided following a ":" operator. This only works for specific
// errors such as a nil object reference, invalid type, unknown
// structure member, or divide-by-zero.
//
// This is only supported when extensions are enabled.
func (c *Compiler) optional() *errors.EgoError {
	if !c.extensionsEnabled {
		return c.newError(errors.ErrUnexpectedToken).Context("?")
	}

	catch := c.b.Mark()
	c.b.Emit(bytecode.Try)

	// What errors do we permit here?
	c.b.Emit(bytecode.WillCatch, bytecode.OptionalCatchSet)

	err := c.unary()
	if !errors.Nil(err) {
		return err
	}

	toEnd := c.b.Mark()
	c.b.Emit(bytecode.Branch)
	_ = c.b.SetAddressHere(catch)

	if !c.t.IsNext(":") {
		return c.newError(errors.ErrMissingCatch)
	}

	err = c.unary()
	if !errors.Nil(err) {
		return err
	}

	_ = c.b.SetAddressHere(toEnd)
	c.b.Emit(bytecode.TryPop)

	return nil
}
