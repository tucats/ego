package compiler

import "github.com/tucats/ego/errors"

// Assignment compiles an assignment statement.
func (c *Compiler) Assignment() error {
	storeLValue, err := c.LValue()
	if err != nil {
		return err
	}

	if !c.t.AnyNext(":=", "=", "<-") {
		return c.NewError(errors.MissingAssignmentError)
	}

	expressionCode, err := c.Expression()
	if err != nil {
		return err
	}

	c.b.Append(expressionCode)
	c.b.Append(storeLValue)

	return nil
}
