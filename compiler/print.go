package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compilePrint compiles the "print" extension statement (not part of standard
// Go; available only when extensions are enabled). The keyword has already been
// consumed by the caller.
//
// Zero or more comma-separated expressions are compiled. Each expression is
// collected into a temporary slice so the resulting bytecode can be appended in
// reverse order — this is necessary because the runtime Print instruction pops
// values off the stack in LIFO order and must receive them in argument order.
//
// A Print instruction is emitted with the argument count as its operand.
// If the argument list did not end with a trailing comma (i.e. newline == true),
// a Newline instruction is also emitted so the output ends with "\n".
func (c *Compiler) compilePrint() error {
	newline := true
	count := 0

	expressions := []*bytecode.ByteCode{}

	for !c.isStatementEnd() {
		if c.t.IsNext(tokenizer.CommaToken) {
			return c.compileError(errors.ErrUnexpectedToken, c.t.Peek(1))
		}

		bc, err := c.Expression(true)
		if err != nil {
			return err
		}

		newline = true

		expressions = append(expressions, bc)

		//		c.b.Append(bc)

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
