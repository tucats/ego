package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
)

// Expression is the public entrypoint to compile an expression which
// returns a bytecode segment as it's result. This lets code compile
// an expression, but save the generated code to emit later.
//
// The function grammar considers a conditional to be the top of the
// parse tree, so we start evaluating there.
//
// From the golang doc, operator precedence is:
//
//  Precedence    Operator
//	5             *  /  %  <<  >>  &  &^
//	4             +  -  |  ^
//	3             ==  !=  <  <=  >  >=
//	2             &&
//	1             ||

func (c *Compiler) Expression() (*bytecode.ByteCode, *errors.EgoError) {
	cx := New("expression eval")
	cx.t = c.t
	cx.b = bytecode.New("subexpression")
	cx.Types = c.Types

	err := cx.conditional()
	if errors.Nil(err) {
		c.t = cx.t
	}

	return cx.b, err
}
