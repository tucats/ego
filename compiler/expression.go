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
//
// If the "reportUsageErrors" flag is set, this compilation will fail for compiler errors
// involving invalid variable references. This is normally true. It can be set to false when
// this call is used to determine if a given expression _could_ be valid as an expression without
// expecting to generate any code.
func (c *Compiler) Expression(reportUsageErrors bool) (*bytecode.ByteCode, error) {
	cx := c.Clone("expression eval")

	err := cx.conditional()
	if err == nil {
		c.t = cx.t
		c.flags = cx.flags
		c.scopes = cx.scopes

		if reportUsageErrors {
			return cx.Close()
		} else {
			return c.b, nil
		}
	}

	return nil, err
}

// emitExpression is a helper function for compiling an expression and
// immediately emitting the code into the associated bytecode stream.
// If an error occurs, the error is returned and no code is added to the
// bytecode stream.
func (c *Compiler) emitExpression() error {
	bc, err := c.Expression(true)
	if err != nil {
		return err
	}

	c.b.Append(bc)

	return nil
}
