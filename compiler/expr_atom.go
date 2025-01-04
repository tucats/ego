package compiler

import (
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

var anonStructCount atomic.Int32

func (c *Compiler) expressionAtom() error {
	t := c.t.Peek(1)

	// Is it a short-form try/catch?
	if t == tokenizer.OptionalToken {
		c.t.Advance(1)

		return c.optional()
	}

	// Is it a radix constant value? If so, convert to decimal.
	t, _ = convertRadixToDecimal(t)
	text := t.Spelling()

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
		return c.compileAddressOf()
	}

	// Is this dereference?
	if t == tokenizer.PointerToken {
		return c.compilePointerDereference()
	}

	// Is this a parenthesis expression?
	if done, err := c.compileSubExpressions(t); done {
		return err
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

		id := c.t.Peek(2)
		colon := c.t.Peek(3)

		// Is it a declared structure format?
		if id.IsIdentifier() && colon != tokenizer.ColonToken {
			return c.parseStructDeclaration()
		}

		return c.parseStruct()
	}

	// If the token is a number, convert it to the most precise type
	// an dpush on the stack.
	if t.IsClass(tokenizer.IntegerTokenClass) {
		return pushIntConstant(c, t)
	}

	if t.IsClass(tokenizer.FloatTokenClass) {
		if i, err := strconv.ParseFloat(text, 64); err == nil {
			c.t.Advance(1)
			c.b.Emit(bytecode.Push, data.Constant(i))

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

	// Is it a rune?
	if shouldReturn, err := c.compileRuneExpression(t); shouldReturn {
		return err
	}

	// Is the token some other kind of value object?
	if t.IsValue() {
		c.t.Advance(1)
		c.b.Emit(bytecode.Push, t)

		return nil
	}

	// Is it a type cast?
	if c.t.Peek(2) == tokenizer.StartOfListToken {
		mark := c.t.Mark()

		// Skip the parentheses
		err := c.compileTypeCast()
		if err == nil {
			return nil
		}

		c.t.Set(mark)
	}

	// Watch out for function calls here.
	isPanic := c.flags.extensionsEnabled && (c.t.Peek(1) == tokenizer.PanicToken)
	if !isPanic && c.t.Peek(2) != tokenizer.StartOfListToken {
		marker := c.t.Mark()

		if typeSpec, err := c.parseType("", true); err == nil {
			// Is there an initial value for the type?
			if !typeSpec.IsBaseType() && c.t.Peek(1) == tokenizer.DataBeginToken {
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
		return c.compileSymbolValue(t)
	}

	// Not something we know what to do with...
	return c.error(errors.ErrUnexpectedToken, t)
}

// Given a token and an optional post-increment or decrement operation,
// generate the code to load the value on the stack and handle the
// option if any.
func (c *Compiler) compileSymbolValue(t tokenizer.Token) error {
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
	c.UseVariable(t.Spelling())

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

// For a token of class Integer, generate the code to push a constant
// instance of the integer value, in either int, or int64 sizes based
// on the size of the value.
func pushIntConstant(c *Compiler, t tokenizer.Token) error {
	var (
		i   int64
		err error
	)

	text := t.Spelling()

	if i, err = strconv.ParseInt(text, 10, 32); err == nil {
		c.t.Advance(1)
		c.b.Emit(bytecode.Push, data.Constant(int(i)))

		return nil
	}

	if i, err = strconv.ParseInt(text, 10, 64); err == nil {
		c.t.Advance(1)
		c.b.Emit(bytecode.Push, data.Constant(i))

		return nil
	}

	return err
}

// Compile a parenthiated subexpression. This requires that the expression
// be surrounded by parentheses, and the tokenizer is positioned after the
// closing parenthesis.
func (c *Compiler) compileSubExpressions(t tokenizer.Token) (bool, error) {
	if t == tokenizer.StartOfListToken {
		c.t.Advance(1)

		if err := c.conditional(); err != nil {
			return true, err
		}

		if c.t.Next() != tokenizer.EndOfListToken {
			return true, c.error(errors.ErrMissingParenthesis)
		}

		return true, nil
	}

	return false, nil
}

// For a given token, if it contains an integer representation of a radix
// integer value (0x for binary, 0o for octal, 0x for hexadecimal) convert
// the value to a simple decimal integer token.
func convertRadixToDecimal(t tokenizer.Token) (tokenizer.Token, error) {
	var (
		err   error
		radix int
		value int64
	)

	// If it was a quoted string, don't mess with it.
	if t.IsString() {
		return t, nil
	}

	text := t.Spelling()
	offset := 2

	if strings.HasPrefix(strings.ToLower(text), "0b") {
		radix = 2
	} else if strings.HasPrefix(strings.ToLower(text), "0o") {
		radix = 8
	} else if strings.HasPrefix(strings.ToLower(text), "0x") {
		radix = 16
	} else {
		// Not a radix value, so bail out
		return t, nil
	}

	value, err = strconv.ParseInt(text[offset:], radix, 32)

	if err != nil {
		return t, errors.ErrInvalidInteger.Context(text)
	}

	return tokenizer.NewIntegerToken(strconv.Itoa(int(value))), nil
}

func (c *Compiler) compileRuneExpression(t tokenizer.Token) (bool, error) {
	s := t.Spelling()
	if len(s) > 1 && s[0] == '\'' && s[len(s)-1] == '\'' {
		runes := []rune{}

		// Scan over the string one character/rune at a time. Ignore the first
		// and last characters which as single quote characters.
		for index, r := range s {
			if index == 0 || index == len(s)-1 && r == '\'' {
				continue
			}

			runes = append(runes, r)
		}

		c.t.Advance(1)

		// Normal case is a single rune, which is pushed on the stack.
		if len(runes) == 1 {
			c.b.Emit(bytecode.Push, data.Constant(rune(runes[0])))

			return true, nil
		} else {
			// If it's a string of runes, create a new array of runes and push it on the stack.
			// This is only valid when extensions are enabled.
			if c.flags.extensionsEnabled {
				runeArray := make([]interface{}, len(runes))
				for i, r := range runes {
					runeArray[i] = r
				}

				d := data.NewArrayFromInterfaces(data.Int32Type, runeArray...)
				c.b.Emit(bytecode.Push, d)

				return true, nil
			} else {
				return true, c.error(errors.ErrInvalidRune).Context(s)
			}
		}
	}

	return false, nil
}

func (c *Compiler) compileTypeCast() error {
	var (
		err      error
		typeSpec *data.Type
	)

	if typeSpec, err = c.parseType("", true); err == nil {
		if c.t.IsNext(tokenizer.StartOfListToken) {
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

	return err
}

func (c *Compiler) compilePointerDereference() error {
	c.t.Advance(1)

	// If it's dereference of a symbol, short-circuit that
	if c.t.Peek(1).IsIdentifier() {
		name := c.t.Next()
		c.UseVariable(name.Spelling())
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

func (c *Compiler) compileAddressOf() error {
	c.t.Advance(1)

	// If it's address of a symbol, short-circuit that. We do not allow
	// readonly symbols to be addressable.
	if c.t.Peek(1).IsIdentifier() {
		name := c.t.Next()
		if strings.HasPrefix(name.Spelling(), defs.ReadonlyVariablePrefix) {
			return c.error(errors.ErrReadOnlyAddressable, name)
		}

		// If it's a type, is this an address of an initializer for a type?
		if t, found := c.types[name.Spelling()]; found && c.t.Peek(1) == tokenizer.DataBeginToken {
			if err := c.compileInitializer(t); err != nil {
				return err
			} else {
				tempName := data.GenerateName()

				c.b.Emit(bytecode.StoreAlways, tempName)
				c.b.Emit(bytecode.AddressOf, tempName)

				return nil
			}
		}

		c.UseVariable(name.Spelling())
		c.b.Emit(bytecode.AddressOf, name.Spelling())
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

func (c *Compiler) parseArray() error {
	var (
		err            error
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
		if wasRange, err := c.compileArrayRangeInitializer(); wasRange {
			return err
		}
	}

	if listTerminator == tokenizer.EmptyToken {
		return nil
	}

	// Handle array initializer values.
	return c.compileArrayInitializer(listTerminator, kind)
}

func (c *Compiler) compileArrayRangeInitializer() (bool, error) {
	var err error

	t1 := 1

	if c.t.Peek(1) == tokenizer.ColonToken {
		err = nil

		c.t.Advance(-1)
	} else {
		t1, err = egostrings.Atoi(c.t.PeekText(1))
		if err != nil {
			err = c.error(err)
		}
	}

	if err == nil {
		if c.t.Peek(2) == tokenizer.ColonToken {
			t2, err := egostrings.Atoi(c.t.PeekText(3))
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
					return true, c.error(errors.ErrInvalidRange)
				}

				return true, nil
			}
		}
	}

	return false, err
}

func (c *Compiler) compileArrayInitializer(listTerminator tokenizer.Token, kind *data.Type) error {
	count := 0

	for c.t.Peek(1) != listTerminator {
		if err := c.conditional(); err != nil {
			return err
		}

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

// Parse a structure definition, followed by an initializer block.
func (c *Compiler) parseStructDeclaration() error {
	var (
		err       error
		t         *data.Type
		savedMark = c.t.Mark()
	)

	// Back up to the "struct" keyword.
	c.t.Set(savedMark - 1)

	// First, parse the structure definition.
	anonStructCount.Add(1)
	name := fmt.Sprintf("<anon type %d>", anonStructCount.Load())

	if t, err = c.parseType(name, true); err != nil {
		return err
	}

	// Now, parse the initializer block. If it does not compile, back
	// it out because it wasn't an initializer after all.
	savedToken := c.t.Mark()
	savedBytecode := c.b.Mark()

	if initErr := c.compileInitializer(t); initErr != nil {
		c.t.Set(savedToken)
		c.b.Truncate(savedBytecode)
	}

	return err
}

// Parse an anonymous structure. This is a list of name/value pairs
// which result in a Struct data type using the data types of the
// values as the structure field types.
func (c *Compiler) parseStruct() error {
	var (
		listTerminator = tokenizer.DataEndToken
		err            error
		count          int
	)

	c.t.Advance(1)
	c.b.Emit(bytecode.Push, bytecode.NewStackMarker("struct-init"))

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

	if c.t.AtEnd() || c.t.Peek(1) == tokenizer.EndOfListToken || c.t.Peek(1) == tokenizer.SemicolonToken {
		return c.error(errors.ErrMissingExpression)
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

	if c.t.AtEnd() || c.t.Peek(1) == tokenizer.EndOfListToken || c.t.Peek(1) == tokenizer.SemicolonToken {
		return c.error(errors.ErrMissingExpression)
	}

	if err := c.unary(); err != nil {
		return err
	}

	_ = c.b.SetAddressHere(toEnd)
	c.b.Emit(bytecode.TryPop)

	return nil
}
