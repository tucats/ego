package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
)

type TypeDefinition struct {
	tokens []string
	model  interface{}
}

var typeMap = []TypeDefinition{
	{
		[]string{"chan"},
		&datatypes.Channel{},
	},
	{
		[]string{"[", "]", "int"},
		1,
	},
	{
		[]string{"[", "]", "bool"},
		true,
	},
	{
		[]string{"[", "]", "float"},
		0.0,
	},
	{
		[]string{"[", "]", "string"},
		"",
	},
	{
		[]string{"[", "]", "interface", "{", "}"},
		nil,
	},
	{
		[]string{"[", "]", "struct"},
		map[string]interface{}{},
	},
	{
		[]string{"[", "]", "{", "}"},
		map[string]interface{}{},
	},
}

func (c *Compiler) Make() error {
	if !c.t.IsNext("make") {
		return c.NewError(UnexpectedTokenError, c.t.Peek(1))
	}
	if !c.t.IsNext("(") {
		return c.NewError(MissingParenthesisError)
	}
	c.b.Emit(bytecode.Load, "make")
	// is this a channel?
	if c.t.IsNext("chan") {
		c.b.Emit(bytecode.Push, &datatypes.Channel{})
	} else {
		found := false
		for _, typeDef := range typeMap {
			found = true
			for pos, token := range typeDef.tokens {
				if c.t.Peek(1+pos) != token {
					found = false
				}
			}
			if found {
				c.t.Advance(len(typeDef.tokens))
				c.b.Emit(bytecode.Push, typeDef.model)

				break
			}
		}
		if !found {
			return c.NewError(InvalidTypeSpecError)
		}
	}
	if !c.t.IsNext(",") {
		return c.NewError(InvalidListError)
	}
	bc, err := c.Expression()
	c.b.Append(bc)
	c.b.Emit(bytecode.Call, 2)
	if err == nil && !c.t.IsNext(")") {
		err = c.NewError(MissingParenthesisError)
	}

	return err
}
