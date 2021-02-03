package compiler

import (
	"github.com/google/uuid"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/tokenizer"
)

// Var compiles the var statement
func (c *Compiler) Var() error {
	names := []string{}

	for {
		name := c.t.Next()
		if !tokenizer.IsSymbol(name) {
			c.t.Advance(-1)

			return c.NewError(InvalidSymbolError, name)
		}
		// See if it's a reserved word.
		if tokenizer.IsReserved(name, c.extensionsEnabled) {
			c.t.Advance(-1)
			// If we mid-list, then just done with list
			if len(names) > 0 {
				break
			}

			return c.NewError(InvalidSymbolError, name)
		}
		name = c.Normalize(name)
		names = append(names, name)
		if !c.t.IsNext(",") {
			break
		}
	}

	// We'll need to use this token string over and over for each name
	// in the list, so remember where to start.
	mark := c.t.Mark()
	for _, name := range names {
		c.t.Set(mark)
		if c.t.Peek(1) == "[" && c.t.Peek(2) == "]" {
			c.t.Advance(2)
			c.b.Emit(bytecode.Array, 0)
		} else {
			typename := c.t.Next()
			switch typename {
			case "chan":
				channel := datatypes.NewChannel(1)
				c.b.Emit(bytecode.Push, channel)

			case "int":
				c.b.Emit(bytecode.Push, int(0))

			case "float", "double":
				c.b.Emit(bytecode.Push, 0.0)

			case "string":
				c.b.Emit(bytecode.Push, "")

			case "bool":
				c.b.Emit(bytecode.Push, false)

			case "uuid":
				c.b.Emit(bytecode.Push, uuid.Nil)

			// Must be a user type name
			default:
				c.b.Emit(bytecode.Load, "new")
				c.b.Emit(bytecode.Load, typename)
				c.b.Emit(bytecode.Call, 1)
			}
		}
		c.b.Emit(bytecode.SymbolCreate, name)
		c.b.Emit(bytecode.Store, name)
	}

	return nil
}
