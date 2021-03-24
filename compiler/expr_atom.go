package compiler

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
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

	// Is this a map declaration?
	if t == "map" && c.t.Peek(2) == "[" {
		return c.mapDeclaration()
	}

	// Is this the "nil" constant?
	if t == "nil" {
		c.t.Advance(1)
		c.b.Emit(bytecode.Push, nil)

		return nil
	}

	// Is an interface?
	if t == "interface{}" {
		c.t.Advance(1)
		c.b.Emit(bytecode.Push, map[string]interface{}{
			datatypes.MetadataKey: map[string]interface{}{
				datatypes.TypeMDKey: "interface{}",
			}})

		return nil
	}

	// Is an empty struct?
	if t == "{}" {
		c.t.Advance(1)
		c.b.Emit(bytecode.Push, map[string]interface{}{
			"__metadata": map[string]interface{}{
				datatypes.TypeMDKey:     "struct",
				datatypes.BasetypeMDKey: "map",
				datatypes.MembersMDKey:  []interface{}{},
				datatypes.ReplicaMDKey:  0,
				datatypes.StaticMDKey:   false,
			},
		})

		return nil
	}

	// Is this a function definition?
	if t == "func" && c.t.Peek(2) == "(" {
		c.t.Advance(1)

		return c.compileFunctionDefinition(true)
	}

	// Is this address-of?
	if t == "&" && tokenizer.IsSymbol(c.t.Peek(2)) {
		name := c.normalize(c.t.Peek(2))
		c.b.Emit(bytecode.AddressOf, name)
		c.t.Advance(2)

		return nil
	}

	// Is this dereference?
	if t == "*" && tokenizer.IsSymbol(c.t.Peek(2)) {
		name := c.normalize(c.t.Peek(2))
		c.b.Emit(bytecode.DeRef, name)
		c.t.Advance(2)

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
			return c.newError(errors.MissingParenthesisError)
		}

		return nil
	}

	// Is this an array constant?
	if t == "[" {
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

	if t == "true" || t == "false" {
		c.t.Advance(1)
		c.b.Emit(bytecode.Push, (t == "true"))

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

	if tokenizer.IsSymbol(t) {
		c.t.Advance(1)

		t = c.normalize(t)
		// Is it a generator for a type?
		if c.t.Peek(1) == "{" && tokenizer.IsSymbol(c.t.Peek(2)) && c.t.Peek(3) == ":" {
			c.b.Emit(bytecode.Load, t)
			c.b.Emit(bytecode.LoadIndex, "__type")
			c.b.Emit(bytecode.Push, "__type")

			err := c.expressionAtom()
			if !errors.Nil(err) {
				return err
			}

			i := c.b.Opcodes()
			ix := i[len(i)-1]
			ix.Operand = util.GetInt(ix.Operand) + 1
			i[len(i)-1] = ix

			return nil
		}

		if c.t.IsNext("{}") {
			c.b.Emit(bytecode.Load, "new")
			c.b.Emit(bytecode.Load, t)
			c.b.Emit(bytecode.Call, 1)
		} else {
			c.b.Emit(bytecode.Load, t)
		}

		return nil
	}

	return c.newError(errors.UnexpectedTokenError, t)
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

	if kind != datatypes.UndefinedTypeDef {
		if !kind.IsArray() {
			return c.newError(errors.InvalidTypeNameError)
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
			return c.newError(errors.MissingBlockError)
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
						return c.newError(errors.InvalidRangeError)
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
		if kind != datatypes.UndefinedTypeDef {
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
			return c.newError(errors.InvalidListError)
		}

		c.t.Advance(1)
	}

	if kind != datatypes.UndefinedTypeDef {
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
				return c.newError(errors.InvalidSymbolError, name)
			}
		}

		name = c.normalize(name)

		// Second element: colon
		if c.t.Next() != ":" {
			return c.newError(errors.MissingColonError)
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
			return c.newError(errors.InvalidListError)
		}

		c.t.Advance(1)
	}

	c.b.Emit(bytecode.Struct, count)
	c.t.Advance(1)

	return errors.New(err)
}

func (c *Compiler) unLit(s string) (string, *errors.EgoError) {
	quote := s[0:1]
	if s[len(s)-1:] != quote {
		return s[1:], c.newError(errors.BlockQuoteError, quote)
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
		return c.newError(errors.UnexpectedTokenError).Context("?")
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
		return c.newError(errors.MissingCatchError)
	}

	err = c.unary()
	if !errors.Nil(err) {
		return err
	}

	_ = c.b.SetAddressHere(toEnd)
	c.b.Emit(bytecode.TryPop)

	return nil
}
