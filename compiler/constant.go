package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// Constant compiles a constant block
func (c *Compiler) Constant() error {

	terminator := ""

	if c.t.IsNext("(") {
		terminator = ")"
	}

	for terminator == "" || !c.t.IsNext(terminator) {
		name := c.t.Next()
		if !tokenizer.IsSymbol(name) {
			return c.NewError(InvalidSymbolError)
		}
		name = c.Normalize(name)

		if !c.t.IsNext("=") {
			return c.NewError(MissingEqualError)
		}
		vx, err := c.Expression()
		if err != nil {
			return err
		}

		// Search to make sure it doesn't contain a load statement that isn't for another
		// constant

		for _, i := range vx.Opcodes() {
			if i.Operation == bytecode.Load && !util.InList(util.GetString(i.Operand), c.constants...) {
				return c.NewError(InvalidConstantError)
			}
		}
		c.constants = append(c.constants, name)

		c.b.Append(vx)
		c.b.Emit(bytecode.Constant, name)

		if terminator == "" {
			break
		}

	}
	return nil
}
