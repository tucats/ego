package compiler

// Assignment compiles an assignment statement.
func (c *Compiler) Assignment() error {

	storeLValue, err := c.LValue()
	if err != nil {
		return err
	}

	if !c.t.AnyNext(":=", "=", "<-") {
		return c.NewError(MissingAssignmentError)
	}

	expressionCode, err := c.Expression()
	if err != nil {
		return err
	}
	c.b.Append(expressionCode)
	c.b.Append(storeLValue)

	return nil

}
