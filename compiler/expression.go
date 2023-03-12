package compiler

import (
	"github.com/tucats/ego/bytecode"
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
//	 Precedence    Operator
//		5             *  /  %  <<  >>  &  &^
//		4             +  -  |  ^
//		3             ==  !=  <  <=  >  >=
//		2             &&
//		1             ||
func (c *Compiler) Expression() (*bytecode.ByteCode, error) {
	cx := New("expression eval")
	cx.t = c.t
	cx.flags = c.flags
	cx.b = bytecode.New("subexpression")
	cx.types = c.types
	cx.sourceFile = c.sourceFile

	err := cx.conditional()
	if err == nil {
		c.t = cx.t
	}

	return cx.b, err
}

// emitExpression is a helper function for compiling an expression and
// immediately emitting the code into the associated bytecode stream.
// If an error occurs, the error is returned and no code is added to the
// bytecode steram.
func (c *Compiler) emitExpression() error {
	bc, err := c.Expression()
	if err != nil {
		return err
	}

	c.b.Append(bc)

	return nil
}
