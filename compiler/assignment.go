package compiler

import "github.com/tucats/ego/errors"

// Assignment compiles an assignment statement.
func (c *Compiler) Assignment() *errors.EgoError {
	storeLValue, err := c.LValue()
	if !errors.Nil(err) {
		return err
	}

	if !c.t.AnyNext(":=", "=", "<-") {
		return c.NewError(errors.MissingAssignmentError)
	}

	expressionCode, err := c.Expression()
	if !errors.Nil(err) {
		return err
	}

	c.b.Append(expressionCode)
	c.b.Append(storeLValue)

	return nil
}
