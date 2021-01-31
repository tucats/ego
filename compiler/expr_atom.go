package compiler

import (
	"strconv"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

func (c *Compiler) expressionAtom() error {
	t := c.t.Peek(1)
	// Is this the make() function?
	if t == "make" && c.t.Peek(2) == "(" {
		return c.Make()
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

		return c.Function(true)
	}

	// Is this a parenthesis expression?
	if t == "(" {
		c.t.Advance(1)
		err := c.conditional()
		if err != nil {
			return err
		}
		if c.t.Next() != ")" {
			return c.NewError(MissingParenthesisError)
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
	// If the token is a number, convert it
	if i, err := strconv.Atoi(t); err == nil {
		c.t.Advance(1)
		c.b.Emit(bytecode.Push, i)

		return nil
	}

	if i, err := strconv.ParseFloat(t, 64); err == nil {
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

		return err
	}
	if runeValue == "`" {
		c.t.Advance(1)
		s, err := c.unLit(t)
		c.b.Emit(bytecode.Push, s)

		return err
	}
	if tokenizer.IsSymbol(t) {
		c.t.Advance(1)
		t = c.Normalize(t)
		// Is it a generator for a type?
		if c.t.Peek(1) == "{" && tokenizer.IsSymbol(c.t.Peek(2)) && c.t.Peek(3) == ":" {
			c.b.Emit(bytecode.Load, t)
			c.b.Emit(bytecode.Push, "__type")
			c.b.Emit(bytecode.LoadIndex)
			c.b.Emit(bytecode.Push, "__type")
			err := c.expressionAtom()
			if err != nil {
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

	return c.NewError(UnexpectedTokenError, t)
}

func (c *Compiler) parseArray() error {
	var listTerminator = ""
	if c.t.Peek(1) == "(" {
		listTerminator = ")"
	}
	if c.t.Peek(1) == "[" {
		listTerminator = "]"
	}
	if listTerminator == "" {
		return nil
	}
	c.t.Advance(1)
	count := 0
	t1 := 1
	var err error

	// Let's experimenally see if this is a range constant expression. This can be
	// of the form [start:end] which creates an array of integers between the start
	// and end values (inclusive). It can also be of the form [:end] which assumes
	// a start number of 1.
	if c.t.Peek(1) == ":" {
		err = nil
		c.t.Advance(-1)
	} else {
		t1, err = strconv.Atoi(c.t.Peek(1))
	}
	if err == nil {
		if c.t.Peek(2) == ":" {
			t2, err := strconv.Atoi(c.t.Peek(3))
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
				if !c.t.IsNext("]") {
					return c.NewError(InvalidRangeError)
				}

				return nil
			}
		}
	}

	for c.t.Peek(1) != listTerminator {
		err := c.conditional()
		if err != nil {
			return err
		}
		count = count + 1
		if c.t.AtEnd() {
			break
		}
		if c.t.Peek(1) == listTerminator {
			break
		}
		if c.t.Peek(1) != "," {
			return c.NewError(InvalidListError)
		}
		c.t.Advance(1)
	}
	c.b.Emit(bytecode.Array, count)
	c.t.Advance(1)

	return nil
}

func (c *Compiler) parseStruct() error {
	var listTerminator = "}"
	var err error
	c.t.Advance(1)
	count := 0

	for c.t.Peek(1) != listTerminator {
		// First element: name
		name := c.t.Next()
		if len(name) > 2 && name[0:1] == "\"" {
			name, err = strconv.Unquote(name)
			if err != nil {
				return err
			}
		} else {
			if !tokenizer.IsSymbol(name) {
				return c.NewError(InvalidSymbolError, name)
			}
		}
		name = c.Normalize(name)
		// Second element: colon
		if c.t.Next() != ":" {
			return c.NewError(MissingColonError)
		}

		// Third element: value, which is emitted.
		err := c.conditional()
		if err != nil {
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
			return c.NewError(InvalidListError)
		}
		c.t.Advance(1)
	}
	c.b.Emit(bytecode.Struct, count)
	c.t.Advance(1)

	return err
}

func (c *Compiler) unLit(s string) (string, error) {
	quote := s[0:1]
	if s[len(s)-1:] != quote {
		return s[1:], c.NewError(BlockQuoteError, quote)
	}

	return s[1 : len(s)-1], nil
}
