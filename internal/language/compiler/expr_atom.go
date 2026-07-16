package compiler

import (
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/tokenizer"
	"github.com/tucats/ego/internal/util/strings"
)

var anonStructCount atomic.Int32

func (c *Compiler) expressionAtom() error {
	t := c.t.Peek(1)

	// Is it a macro invocation?
	if t.Is(tokenizer.DirectiveToken) {
		name := c.t.Peek(2)
		if name.IsIdentifier() {
			mark := c.t.Mark()
			c.t.Advance(2)

			if err := c.compilerMacro(name.Spelling(), true); err != nil {
				return err
			}

			// Now that we have processed the macro, back up and try the expression atom
			// again.
			c.t.Set(mark)

			return c.expressionAtom()
		}
	}

	// Is it an IF/THEN/ELSE expression? Requires extensions to be enabled...
	if t.Is(tokenizer.IfToken) && c.flags.extensionsEnabled {
		c.t.Advance(1)

		return c.ifExpression()
	}

	// Is it a short-form try/catch?
	if t.Is(tokenizer.OptionalToken) {
		c.t.Advance(1)

		return c.optional()
	}

	// Is it a radix constant value? If so, convert to decimal.
	t, _ = convertRadixToDecimal(t)
	text := t.Spelling()

	// Is this the "nil" constant?
	if t.Is(tokenizer.NilToken) {
		c.t.Advance(1)
		c.b.Emit(bytecode.Push, nil)

		return nil
	}

	// Is an empty struct?
	if t.Is(tokenizer.EmptyInitializerToken) {
		c.t.Advance(1)
		c.b.Emit(bytecode.Push, data.NewStruct(data.StructType).SetStatic(false))

		return nil
	}

	// Is this a function definition?
	if t.Is(tokenizer.FuncToken) && c.t.Peek(2).Is(tokenizer.StartOfListToken) {
		c.t.Advance(1)

		return c.compileFunctionDefinition(true)
	}

	// Is this address-of?
	if t.Is(tokenizer.AddressToken) {
		return c.compileAddressOf()
	}

	// Is this dereference?
	if t.Is(tokenizer.PointerToken) {
		return c.compilePointerDereference()
	}

	// Is this a channel receive used as a general expression atom, e.g.
	// fmt.Println(<-ch) or 10 + <-ch, as opposed to the direct right-hand
	// side of an assignment statement (v := <-ch, v, ok := <-ch), which
	// assignment.go/lvalue.go already handled before this fix (BUG-62)?
	if t.Is(tokenizer.ChannelReceiveToken) {
		return c.compileChannelReceive()
	}

	// Is this a parenthesis expression?
	if done, err := c.compileSubExpressions(t); done {
		return err
	}

	// Is this an array constant?
	if t.Is(tokenizer.StartOfArrayToken) && !(c.t.Peek(2).Is(tokenizer.EndOfArrayToken)) {
		return c.parseArray()
	}

	// Is it a map constant?
	if t.Is(tokenizer.DataBeginToken) {
		return c.parseStruct(true)
	}

	if t.Is(tokenizer.StructToken) && c.t.Peek(2).Is(tokenizer.DataBeginToken) {
		c.t.Advance(1)

		id := c.t.Peek(2)
		colon := c.t.Peek(3)

		// Is it a declared structure format?
		if id.IsIdentifier() && colon.IsNot(tokenizer.ColonToken) {
			return c.parseStructDeclaration()
		}

		return c.parseStruct(true)
	}

	// If the token is a number, convert it to the most precise type
	// and push on the stack.
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

	// An imaginary literal (e.g. "3i", "2.5i") always compiles to a
	// complex128 constant, matching Go's rule that an untyped imaginary
	// constant defaults to complex128 regardless of magnitude -- the same
	// way a plain float literal always defaults to float64 above.
	if t.IsClass(tokenizer.ComplexTokenClass) {
		numericText := strings.TrimSuffix(text, "i")

		if i, err := strconv.ParseFloat(numericText, 64); err == nil {
			c.t.Advance(1)
			c.b.Emit(bytecode.Push, data.Constant(complex(0, i)))

			return nil
		}

		return c.compileError(errors.ErrInvalidValue).Context(text)
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

	// Is it a type cast? Skipped when the token is a built-in type keyword
	// (t.IsType()) that has ALSO been shadowed by a local variable (BUG-75)
	// -- `int := 5; int(3)` must not be read as a cast to the (now-shadowed)
	// `int` type, matching Go's rule that a local declaration hides the
	// predeclared type of the same name for the rest of its scope. Falling
	// through here lets the normal identifier/call handling below treat
	// "int" as the variable it now is. The t.IsType() gate matters: a
	// user-defined type name (e.g. "Point") is never TypeTokenClass, but IS
	// separately registered in the current scope by typeEmitter purely for
	// "unused type" tracking -- without gating on t.IsType(), isLocalSymbol
	// would misread that self-registration as "Point has been shadowed" and
	// break ordinary `Point(x)` conversions and `Point{...}` literals.
	if c.t.Peek(2).Is(tokenizer.StartOfListToken) && !(t.IsType() && c.isLocalSymbol(text)) {
		mark := c.t.Mark()

		// Skip the parentheses
		err := c.compileTypeCast()
		if err == nil {
			return nil
		}

		c.t.Set(mark)
	}

	// Watch out for function calls here.
	//
	// If this token is a built-in type keyword (t.IsType()) that has ALSO
	// been declared as a local variable (or parameter) somewhere still in
	// scope, the variable shadows the built-in type -- Go's ordinary
	// scoping rule. Skip the "parse this as a bare type reference" attempt
	// entirely in that case so the identifier falls through to the plain
	// symbol lookup below, which loads the variable instead of pushing the
	// type object (BUG-75). Without this check, `int := 5;
	// fmt.Println(int)` printed the type "int" instead of the value 5.
	//
	// The t.IsType() gate is required, not just a nice-to-have: a
	// user-defined type name is also registered in the current scope (by
	// typeEmitter, purely for "unused type" tracking) the moment its `type
	// X ...` declaration is compiled. Without gating on t.IsType(),
	// isLocalSymbol would treat that self-registration as if the type had
	// been shadowed by a variable of its own name, breaking ordinary
	// `Point{...}` struct literals and `Point(x)` conversions immediately
	// after declaring `type Point ...`.
	isPanic := c.flags.extensionsEnabled && (c.t.Peek(1).Is(tokenizer.PanicToken))
	if !isPanic && c.t.Peek(2).IsNot(tokenizer.StartOfListToken) && !(t.IsType() && c.isLocalSymbol(text)) {
		marker := c.t.Mark()

		if typeSpec, err := c.parseType("", true); err == nil {
			// Mark the type as having been used if it's a user type name.
			if typeSpec.Kind() == data.TypeType.Kind() && typeSpec.Package() == "" {
				if err = c.ReferenceSymbol(typeSpec.Name()); err != nil {
					return err
				}
			}

			// Is there an initial value for the type?
			if !typeSpec.IsBaseType() && c.t.Peek(1).Is(tokenizer.DataBeginToken) {
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
		} else if errors.Equals(err, errors.ErrChannelElementType) {
			// "chan T" (e.g. inside make(chan string, 10)) is never
			// ambiguous with a legitimate identifier reference -- unlike
			// every other parseType failure here, which falls back to a
			// plain symbol lookup on the assumption that the token just
			// isn't a type name at all, propagate this one immediately.
			// Otherwise "chan" would be silently re-parsed as an ordinary
			// identifier reference, leaving the element-type token (e.g.
			// "string") behind as an unconsumed, confusing leftover that
			// surfaces as an unrelated "invalid list" error from the
			// enclosing argument list parser instead (BUG-72).
			return err
		}

		c.t.Set(marker)
	}

	// Is it a recover() call? recover() is only meaningful inside a deferred
	// function that runs during panic unwinding; the Recover opcode handles the
	// semantics. It takes no arguments and returns the panic value (or nil).
	if t.Is(tokenizer.RecoverToken) {
		c.t.Advance(1)

		if !c.t.IsNext(tokenizer.StartOfListToken) {
			return errors.ErrMissingParenthesis
		}

		if !c.t.IsNext(tokenizer.EndOfListToken) {
			return errors.ErrMissingParenthesis
		}

		c.b.Emit(bytecode.Recover)

		return nil
	}

	// Is this a reference to Go's predeclared "iota" identifier? It is not a
	// reserved word -- there is no dedicated token type for it -- so we detect
	// it here by comparing the identifier's spelling, the same way the "true"
	// and "false" boolean literals are recognized above by comparing spelling
	// against defs.True/defs.False.
	//
	// iota only means something inside a const(...) declaration. compileConst()
	// (see constant.go) sets c.iota to 0 or higher for the duration of
	// compiling each constant's right-hand-side expression, and restores it to
	// -1 once the declaration is finished. So c.iota >= 0 here means "we are
	// currently compiling the right-hand side of a ConstSpec", and c.iota
	// itself holds that spec's zero-based position in the block. When that's
	// true we push the current counter value as an ordinary integer literal
	// (exactly like a numeric literal such as `2` would be pushed) instead of
	// falling through to compileSymbolValue(), which would otherwise treat
	// "iota" as an ordinary variable reference and fail with an undefined-symbol
	// error. Outside of a const declaration (c.iota == -1) we deliberately do
	// nothing special here, so a plain, unrelated use of the name "iota" falls
	// through to the normal identifier lookup below and reports "undefined
	// symbol", matching Go's own behavior.
	if text == defs.Iota && c.iota >= 0 {
		c.t.Advance(1)
		c.b.Emit(bytecode.Push, data.Constant(c.iota))

		return nil
	}

	// Is it just a symbol needing a load?
	if t.IsIdentifier() {
		// Check for auto-increment or decrement
		return c.compileSymbolValue(t)
	}

	// Not something we know what to do with...
	return c.compileError(errors.ErrUnexpectedToken, t)
}

// Given a token and an optional post-increment or decrement operation,
// generate the code to load the value on the stack and handle the
// option if any.
func (c *Compiler) compileSymbolValue(t tokenizer.Token) error {
	autoMode := bytecode.NoOperation

	if c.t.Peek(2).Is(tokenizer.IncrementToken) {
		autoMode = bytecode.Add
	}

	if c.t.Peek(2).Is(tokenizer.DecrementToken) {
		autoMode = bytecode.Sub
	}

	// If language extensions are supported and this is an auto-increment
	// or decrement operation, do it now. The modification is applied after
	// the value is read; i.e. the atom is the pre-modified value.
	if err := c.ReferenceSymbol(t.Spelling()); err != nil {
		return err
	}

	if c.flags.extensionsEnabled && (autoMode != bytecode.NoOperation) {
		c.emitLoadName(c.b, t.Spelling())
		c.b.Emit(bytecode.Dup)
		c.b.Emit(bytecode.Push, 1)
		c.b.Emit(autoMode)
		c.emitStoreName(c.b, t.Spelling())
		c.t.Advance(2)
	} else {
		c.emitLoadName(c.b, t.Spelling())
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

// Compile a parenthetical sub-expression. This requires that the expression
// be surrounded by parentheses, and the tokenizer is positioned after the
// closing parenthesis.
func (c *Compiler) compileSubExpressions(t tokenizer.Token) (bool, error) {
	if t.Is(tokenizer.StartOfListToken) {
		c.t.Advance(1)

		if err := c.conditional(); err != nil {
			return true, err
		}

		if !c.t.Next().Is(tokenizer.EndOfListToken) {
			return true, c.compileError(errors.ErrMissingParenthesis)
		}

		return true, nil
	}

	return false, nil
}

// For a given token, if it contains an integer representation of a radix
// integer value (0b for binary, 0o for octal, 0x for hexadecimal, or a bare
// leading zero followed by digits for Go's legacy octal notation) convert
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
	} else if len(text) > 1 && text[0] == '0' && text[1] >= '0' && text[1] <= '9' {
		// Go's legacy octal notation: a bare leading zero followed by more
		// digits (e.g. 0644), with no "o"/"x"/"b" radix letter. A lone "0"
		// digit, or a leading zero followed by "." or "e" (a float literal
		// like 0.5 or 0e5), does not match this and is left alone.
		radix = 8
		offset = 1
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

// IS this token a valid rune expression? Note that if the token is really
// a string, it cannot be a rune -- this handles the case of "'A'" being
// incorrectly parsed as a rune value 65, instead of a string containing
// a quote, letter A, and a quote.
func (c *Compiler) compileRuneExpression(t tokenizer.Token) (bool, error) {
	if t.Class() == tokenizer.StringTokenClass {
		return false, nil
	}

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
				runeArray := make([]any, len(runes))
				for i, r := range runes {
					runeArray[i] = r
				}

				d := data.NewArrayFromInterfaces(data.Int32Type, runeArray...)
				c.b.Emit(bytecode.Push, d)

				return true, nil
			} else {
				return true, c.compileError(errors.ErrInvalidRune).Context(s)
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
			b, err := c.Expression(true)
			if err == nil {
				for c.t.IsNext(tokenizer.CommaToken) {
					b2, e2 := c.Expression(true)
					if e2 == nil {
						b.Append(b2)
					} else {
						err = e2

						break
					}
				}
			}

			if err == nil && c.t.Peek(1).Is(tokenizer.EndOfListToken) {
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
		if err := c.ReferenceSymbol(name.Spelling()); err != nil {
			return err
		}

		c.emitDeRefName(c.b, name.Spelling())
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
			return c.compileError(errors.ErrReadOnlyAddressable, name)
		}

		// If it's a type, is this an address of an initializer for a type?
		if t, found := c.types[name.Spelling()]; found && c.t.Peek(1).Is(tokenizer.DataBeginToken) {
			if err := c.compileInitializer(t); err != nil {
				return err
			} else {
				tempName := data.GenerateName()

				c.b.Emit(bytecode.StoreAlways, tempName)
				c.b.Emit(bytecode.AddressOf, tempName)

				return c.ReferenceSymbol(name.Spelling())
			}
		}

		if err := c.ReferenceSymbol(name.Spelling()); err != nil {
			return err
		}

		c.emitAddressOfName(c.b, name.Spelling())
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

// compileChannelReceive compiles a channel receive ("<-ch") appearing as a
// general expression atom -- e.g. fmt.Println(<-ch), or 10 + <-ch -- rather
// than as the direct right-hand side of an assignment statement. The single-
// and two-value assignment forms (v := <-ch, v, ok := <-ch) are unaffected;
// they are handled separately in assignment.go/lvalue.go and do not call
// this function. The "<-" token has already been peeked, but not consumed,
// by the caller (expressionAtom).
//
// c.reference() -- not a full c.Expression -- is used to compile the channel
// operand, so that "<-" binds as tightly as Go's own grammar requires: it
// parses an atom plus any suffix chain (chans[i], obj.ch, getChan()) but
// stops before a following binary operator, so "10 + <-ch" compiles as
// "10 + (<-ch)" rather than swallowing a trailing "+ ..." into the channel
// expression itself.
func (c *Compiler) compileChannelReceive() error {
	c.t.Advance(1)

	if err := c.reference(); err != nil {
		return err
	}

	// ReceiveChannel pops the channel object left on the stack by the
	// reference() call above, and pushes three items:
	// [StackMarker("receive"), ok, datum] (datum on top). That shape exists
	// to satisfy the two-value comma-ok assignment form's StackCheck; as a
	// plain expression atom we only want the received value itself, so
	// collapse the three items down to just datum using the same
	// store-in-temp / DropToMarker / reload idiom optional() uses for the
	// same kind of stack cleanup (see optional(), later in this file).
	c.b.Emit(bytecode.ReceiveChannel)

	tempName := data.GenerateName()
	c.b.Emit(bytecode.CreateAndStore, tempName)
	c.b.Emit(bytecode.DropToMarker, bytecode.NewStackMarker("receive"))
	c.b.Emit(bytecode.Load, tempName)
	c.b.Emit(bytecode.SymbolDelete, tempName)

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
			return c.compileError(errors.ErrInvalidTypeName)
		}

		// Is it an empty declaration, such as []int{} ?
		if c.t.IsNext(tokenizer.EmptyInitializerToken) {
			c.b.Emit(bytecode.Array, 0, kind)

			return nil
		}

		// There better be at least the start of an initialization block then.
		if !c.t.IsNext(tokenizer.DataBeginToken) {
			return c.compileError(errors.ErrMissingBlock)
		}

		listTerminator = tokenizer.DataEndToken
	} else {
		c.t.Set(marker)

		if c.t.Peek(1).Is(tokenizer.StartOfListToken) {
			listTerminator = tokenizer.EndOfListToken
		} else {
			if c.t.Peek(1).Is(tokenizer.StartOfArrayToken) {
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

	if listTerminator.Is(tokenizer.EmptyToken) {
		return nil
	}

	// Handle array initializer values.
	return c.compileArrayInitializer(listTerminator, kind)
}

func (c *Compiler) compileArrayRangeInitializer() (bool, error) {
	var err error

	t1 := 1

	if c.t.Peek(1).Is(tokenizer.ColonToken) {
		err = nil

		c.t.Advance(-1)
	} else {
		t1, err = egostrings.Atoi(c.t.PeekText(1))
		if err != nil {
			err = c.compileError(err)
		}
	}

	if err == nil {
		if c.t.Peek(2).Is(tokenizer.ColonToken) {
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
					return true, c.compileError(errors.ErrInvalidRange)
				}

				return true, nil
			}
		}
	}

	return false, err
}

func (c *Compiler) compileArrayInitializer(listTerminator tokenizer.Token, kind *data.Type) error {
	count := 0

	for {
		c.skipSyntheticSemicolons()

		if c.t.Peek(1).Is(listTerminator) {
			break
		}

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

		c.skipSyntheticSemicolons()

		if c.t.Peek(1).Is(listTerminator) {
			break
		}

		if c.t.Peek(1).IsNot(tokenizer.CommaToken) {
			return c.compileError(errors.ErrInvalidList)
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

// skipSyntheticSemicolons consumes any run of ";" tokens at the current
// tokenizer position.
//
// The tokenizer inserts a synthetic ";" at the end of any source line whose
// last token is one that would end a Go statement (including "}"),
// following Go's own automatic semicolon insertion rules (see
// tokenizer.splitLines). It has no awareness that the line might actually
// be inside a brace- or bracket-enclosed literal list (struct, map, or
// array) rather than a statement block, so when an element of such a list
// -- commonly a nested literal value -- ends a source line, a stray ";" is
// left between that element and the "," or closing bracket the list parser
// expects next.
//
// Every literal-list parsing loop must call this both before checking for
// the list terminator and before checking for the "," separator, so that a
// multi-line literal parses the same as its single-line equivalent (BUG-41).
func (c *Compiler) skipSyntheticSemicolons() {
	for c.t.IsNext(tokenizer.SemicolonToken) {
	}
}

// Parse an anonymous structure. This is a list of name/value pairs
// which result in a Struct data type using the data types of the
// values as the structure field types.
func (c *Compiler) parseStruct(needsMarker bool) error {
	var (
		listTerminator = tokenizer.DataEndToken
		err            error
		count          int
	)

	c.t.Advance(1)

	if needsMarker {
		c.b.Emit(bytecode.Push, bytecode.NewStackMarker("struct-init"))
	}

	for {
		c.skipSyntheticSemicolons()

		if c.t.Peek(1).Is(listTerminator) {
			break
		}

		// First element: name
		name := c.t.Next()
		if !name.IsString() && !name.IsIdentifier() {
			return c.compileError(errors.ErrInvalidSymbolName, name)
		}

		name = c.normalizeToken(name)

		// Second element: colon
		if c.t.Next().IsNot(tokenizer.ColonToken) {
			return c.compileError(errors.ErrMissingColon)
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

		c.skipSyntheticSemicolons()

		if c.t.Peek(1).Is(listTerminator) {
			break
		}

		if c.t.Peek(1).IsNot(tokenizer.CommaToken) {
			return c.compileError(errors.ErrInvalidList)
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
		return c.compileError(errors.ErrUnexpectedToken).Context("?")
	}

	if c.t.AtEnd() || c.t.Peek(1).Is(tokenizer.EndOfListToken) || c.t.Peek(1).Is(tokenizer.SemicolonToken) {
		return c.compileError(errors.ErrMissingExpression)
	}

	catch := c.b.Mark()
	c.b.Emit(bytecode.Try)

	// What errors do we permit here?
	c.b.Emit(bytecode.WillCatch, bytecode.OptionalCatchSet)
	c.b.Emit(bytecode.Push, bytecode.NewStackMarker("try"))

	if err := c.unary(); err != nil {
		return err
	}

	// At this point the value is on the stack, but we need to get rid of the marker.
	// Generate code to hold the value in a temp variable while we drop junk from the
	// stack.

	name := data.GenerateName()
	c.b.Emit(bytecode.CreateAndStore, name)
	c.b.Emit(bytecode.DropToMarker, bytecode.NewStackMarker("try"))
	c.b.Emit(bytecode.Load, name)
	c.b.Emit(bytecode.SymbolDelete, name)

	// Generate code to handle the optional catch.
	toEnd := c.b.Mark()
	c.b.Emit(bytecode.Branch)
	_ = c.b.SetAddressHere(catch)

	if !c.t.IsNext(tokenizer.ColonToken) {
		return c.compileError(errors.ErrMissingCatch)
	}

	if c.t.AtEnd() || c.t.Peek(1).Is(tokenizer.EndOfListToken) || c.t.Peek(1).Is(tokenizer.SemicolonToken) {
		return c.compileError(errors.ErrMissingExpression)
	}

	if err := c.unary(); err != nil {
		return err
	}

	_ = c.b.SetAddressHere(toEnd)
	c.b.Emit(bytecode.TryPop)

	return nil
}
