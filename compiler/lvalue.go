package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// IsLValue peeks ahead to see if this is likely to be an lValue
// object. This is used in cases where the parser might be in an
// otherwise ambiguous state.
func (c *Compiler) IsLValue() bool {
	name := c.t.Peek(1)
	if !tokenizer.IsSymbol(name) {
		return false
	}

	// See if it's a reserved word.
	if tokenizer.IsReserved(name, c.extensionsEnabled) {
		return false
	}

	// Let's look ahead to see if it contains any of the tell-tale
	// characters that indicate an lvalue. This does not say if it
	// is a valid/correct lvalue. We also stop searching at some point.
	for i := 2; i < 100; i = i + 1 {
		t := c.t.Peek(i)
		if util.InList(t, ":=", "=", "<-") {
			return true
		}

		if tokenizer.IsReserved(t, c.extensionsEnabled) {
			return false
		}

		if util.InList(t, "{", ";", tokenizer.EndOfTokens) {
			return false
		}
	}

	return false
}

// Check to see if this is a list of lvalues, which can occur
// in a multi-part assignment.
func lvalueList(c *Compiler) (*bytecode.ByteCode, error) {
	bc := bytecode.New("lvalue list")
	count := 0
	savedPosition := c.t.TokenP
	isLvalueList := false

	bc.Emit(bytecode.StackCheck, 1)

	for {
		name := c.t.Next()
		if !tokenizer.IsSymbol(name) {
			return nil, c.NewError(errors.InvalidSymbolError, name)
		}

		name = c.Normalize(name)
		needLoad := true

		// Until we get to the end of the lvalue...
		for util.InList(c.t.Peek(1), ".", "[") {
			if needLoad {
				bc.Emit(bytecode.Load, name)

				needLoad = false
			}

			err := c.lvalueTerm(bc)
			if err != nil {
				return nil, err
			}
		}

		// Cheating here a bit; this opcode does an optional create
		// if it's not found anywhere in the tree already.
		bc.Emit(bytecode.SymbolOptCreate, name)
		patchStore(bc, name, false)

		count++

		if c.t.Peek(1) == "," {
			c.t.Advance(1)

			isLvalueList = true

			continue
		}

		if util.InList(c.t.Peek(1), "=", ":=", "<-") {
			break
		}
	}

	if isLvalueList {
		// TODO if this is a channel store, then a list is not supported yet.
		if c.t.Peek(1) == "<-" {
			return nil, c.NewError(errors.InvalidChannelList)
		}

		// Patch up the stack size check. We can use the SetAddress
		// operator to do this because it really just updates the
		// integer instruction argument.
		_ = bc.SetAddress(0, count)

		// Also, add an instruction that will drop the marker (nil)
		// value
		bc.Emit(bytecode.DropToMarker)

		return bc, nil
	}

	c.t.TokenP = savedPosition

	return nil, c.NewError(errors.NotAnLValueListError)
}

// LValue compiles the information on the left side of
// an assignment. This information is used later to store the
// data in the named object.
func (c *Compiler) LValue() (*bytecode.ByteCode, error) {
	if bc, err := lvalueList(c); err == nil {
		return bc, nil
	}

	bc := bytecode.New("lvalue")

	name := c.t.Next()
	if !tokenizer.IsSymbol(name) {
		return nil, c.NewError(errors.InvalidSymbolError, name)
	}

	name = c.Normalize(name)
	needLoad := true

	// Until we get to the end of the lvalue...
	for c.t.Peek(1) == "." || c.t.Peek(1) == "[" {
		if needLoad {
			bc.Emit(bytecode.Load, name)

			needLoad = false
		}

		err := c.lvalueTerm(bc)
		if err != nil {
			return nil, err
		}
	}

	// Quick optimization; if the name is "_" it just means
	// discard and we can shortcircuit that.
	if name == "_" {
		bc.Emit(bytecode.Drop, 1)
	} else {
		if c.t.Peek(1) == ":=" {
			bc.Emit(bytecode.SymbolCreate, name)
		}

		patchStore(bc, name, c.t.Peek(1) == "<-")
	}

	return bc, nil
}

// Helper function for LValue processing. If the token stream we are
// generating ends in a LoadIndex, but this is the last part of the
// storagebytecode, convert the last operation to a Store which writes
// the value back.
func patchStore(bc *bytecode.ByteCode, name string, isChan bool) {
	// Is the last operation in the stack referecing
	// a parent object? If so, convert the last one to
	// a store operation.
	ops := bc.Opcodes()

	opsPos := bc.Mark() - 1
	if opsPos > 0 && ops[opsPos].Operation == bytecode.LoadIndex {
		ops[opsPos].Operation = bytecode.StoreIndex
	} else {
		if isChan {
			bc.Emit(bytecode.StoreChan, name)
		} else {
			bc.Emit(bytecode.Store, name)
		}
	}
}

// lvalueTerm parses secondary lvalue operations (array indexes, or struct member dereferences).
func (c *Compiler) lvalueTerm(bc *bytecode.ByteCode) error {
	term := c.t.Peek(1)
	if term == "[" {
		c.t.Advance(1)

		ix, err := c.Expression()
		if err != nil {
			return err
		}

		bc.Append(ix)

		if !c.t.IsNext("]") {
			return c.NewError(errors.MissingBracketError)
		}

		bc.Emit(bytecode.LoadIndex)

		return nil
	}

	if term == "." {
		c.t.Advance(1)

		member := c.t.Next()
		if !tokenizer.IsSymbol(member) {
			return c.NewError(errors.InvalidSymbolError, member)
		}

		bc.Emit(bytecode.Push, c.Normalize(member))
		bc.Emit(bytecode.LoadIndex)

		return nil
	}

	return nil
}
