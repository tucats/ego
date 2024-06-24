package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compilePanic compiles a "panic" statement. The verb is followed
// by an expression in parenthesis. This value is pushed on the stack
// and the panic bytecode issued.
func (c *Compiler) compilePanic() error {
	if !c.t.IsNext(tokenizer.StartOfListToken) {
		return errors.ErrMissingParenthesis
	}

	if c.t.IsNext(tokenizer.EndOfListToken) {
		c.b.Emit(bytecode.Push, "panic() called with no arguments")
		c.b.Emit(bytecode.Panic)
	} else {
		if err := c.expressionAtom(); err != nil {
			return err
		}

		c.b.Emit(bytecode.Panic)

		if !c.t.IsNext(tokenizer.EndOfListToken) {
			return errors.ErrMissingParenthesis
		}
	}

	return nil
}
