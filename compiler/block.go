package compiler

import "github.com/tucats/ego/bytecode"

// Block compiles a statement block. The leading { has already
// been parsed.
func (c *Compiler) Block() error {
	parsing := true
	c.b.Emit(bytecode.PushScope)
	c.blockDepth = c.blockDepth + 1
	for parsing {
		if c.t.IsNext("}") {
			break
		}
		err := c.Statement()
		if err != nil {
			return err
		}

		// Skip over a semicolon if found
		_ = c.t.IsNext(";")
		if c.t.AtEnd() {
			return c.NewError(MissingEndOfBlockError)
		}
	}
	c.b.Emit(bytecode.PopScope)
	c.blockDepth = c.blockDepth - 1

	return nil
}
