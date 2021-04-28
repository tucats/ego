package compiler

import (
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileAssignment compiles an assignment statement.
func (c *Compiler) compileAssignment() *errors.EgoError {
	storeLValue, err := c.assignmentTarget()
	if !errors.Nil(err) {
		return err
	}

	if !c.t.AnyNext(":=", "=", "<-") {
		return c.newError(errors.ErrMissingAssignment)
	}

	if c.t.AnyNext(";", tokenizer.EndOfTokens) {
		return c.newError(errors.ErrMissingExpression)
	}

	// If this is a construct like   x := <-ch   skip over the :=
	_ = c.t.IsNext("<-")

	expressionCode, err := c.Expression()
	if !errors.Nil(err) {
		return err
	}

	c.b.Append(expressionCode)
	c.b.Append(storeLValue)

	return nil
}
