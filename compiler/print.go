package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compilePrint compiles a print statement. The verb is already removed
// from the token stream.
func (c *Compiler) compilePrint() error {
	newline := true
	count := 0

	expressions := []*bytecode.ByteCode{}

	for !c.isStatementEnd() {
		if c.t.IsNext(tokenizer.CommaToken) {
			return c.error(errors.ErrUnexpectedToken, c.t.Peek(1))
		}

		bc, err := c.Expression()
		if err != nil {
			return err
		}

		newline = true

		expressions = append(expressions, bc)

		c.b.Append(bc)

		count++

		if !c.t.IsNext(tokenizer.CommaToken) {
			break
		}

		newline = false
	}

	// Put the expressions in the generated code in reverse order so they
	// are put on the stack in the correct order.
	for index := len(expressions) - 1; index >= 0; index-- {
		c.b.Append(expressions[index])
	}

	c.b.Emit(bytecode.Print, count)

	if newline {
		c.b.Emit(bytecode.Newline)
	}

	return nil
}
