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

	for !c.isStatementEnd() {
		if c.t.IsNext(tokenizer.CommaToken) {
			return c.error(errors.ErrUnexpectedToken, c.t.Peek(1))
		}

		bc, err := c.Expression()
		if err != nil {
			return err
		}

		newline = true

		c.b.Append(bc)
		c.b.Emit(bytecode.Print)

		if !c.t.IsNext(tokenizer.CommaToken) {
			break
		}

		newline = false
	}

	if newline {
		c.b.Emit(bytecode.Newline)
	}

	return nil
}
