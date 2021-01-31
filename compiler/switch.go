package compiler

import (
	"github.com/tucats/ego/bytecode"
)

// Switch compiles a switch statement.
func (c *Compiler) Switch() error {
	fixups := make([]int, 0)
	t := MakeSymbol()

	// Parse the expression to test
	tx, err := c.Expression()
	if err != nil {
		return err
	}
	c.b.Append(tx)
	c.b.Emit(bytecode.SymbolCreate, t)
	c.b.Emit(bytecode.Store, t)

	if !c.t.IsNext("{") {
		return c.NewError(MissingBlockError)
	}

	var defaultBlock *bytecode.ByteCode
	next := 0
	for !c.t.IsNext("}") {
		if next > 0 {
			_ = c.b.SetAddressHere(next)
		}
		// Could be a default statement:
		if c.t.IsNext("default") {
			if !c.t.IsNext(":") {
				return c.NewError(MissingColonError)
			}
			savedBC := c.b
			c.b = bytecode.New("default switch")
			for c.t.Peek(1) != "case" && c.t.Peek(1) != "}" {
				err := c.Statement()
				if err != nil {
					return err
				}
			}
			defaultBlock = c.b
			c.b = savedBC

		} else {
			// Must be a "case" statement:
			if !c.t.IsNext("case") {
				return c.NewError(MissingCaseError)
			}
			cx, err := c.Expression()
			if err != nil {
				return err
			}
			c.b.Append(cx)
			c.b.Emit(bytecode.Load, t)
			c.b.Emit(bytecode.Equal)
			next = c.b.Mark()
			c.b.Emit(bytecode.BranchFalse, 0)
			if !c.t.IsNext(":") {
				return c.NewError(MissingColonError)
			}

			for c.t.Peek(1) != "case" && c.t.Peek(1) != "default" && c.t.Peek(1) != "}" {
				err := c.Statement()
				if err != nil {
					return err
				}
			}

			// Emit the code that will jump to the exit point of the statement
			fixups = append(fixups, c.b.Mark())
			c.b.Emit(bytecode.Branch, 0)
		}
	}

	// If there was a last case with conditional, branch it here.
	if next > 0 {
		_ = c.b.SetAddressHere(next)
	}

	// If there was a default block, emit it here
	if defaultBlock != nil {
		c.b.Append(defaultBlock)
	}

	// Fixup all the jumps to the exit point
	for _, n := range fixups {
		_ = c.b.SetAddressHere(n)
	}
	c.b.Emit(bytecode.SymbolDelete, t)

	return nil
}
