package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// isAssignmentTarget peeks ahead to see if this is likely to be an lValue
// object. This is used in cases where the parser might be in an
// otherwise ambiguous state.
func (c *Compiler) isAssignmentTarget() bool {
	// Remember were we are, and set it back when done.
	mark := c.t.Mark()
	defer c.t.Set(mark)

	// If this is a leading asterisk, that's fine. Eat all the "*" in the string,
	// which covers things like **x=3 and such.
	for c.t.Peek(1).Is(tokenizer.PointerToken) {
		c.t.Advance(1)
	}

	// See if it's a symbol
	if name := c.t.Peek(1); !name.IsIdentifier() {
		return false
	} else {
		// See if it's a reserved word.
		if name.IsReserved(c.flags.extensionsEnabled) {
			return false
		}
	}

	// Let's look ahead to see if it contains any of the tell-tale
	// tokens that indicate an lvalue. This does not determine if it
	// is a valid/correct lvalue. We also stop searching at some point.
	for i := 2; i < 100; i = i + 1 {
		t := c.t.Peek(i)
		if tokenizer.InList(t,
			tokenizer.DefineToken,
			tokenizer.AssignToken,
			tokenizer.ChannelReceiveToken,
			tokenizer.AddAssignToken,
			tokenizer.SubtractAssignToken,
			tokenizer.MultiplyAssignToken,
			tokenizer.DivideAssignToken) {
			return true
		}

		// Is this an auto increment?
		if c.t.Peek(i).Is(tokenizer.IncrementToken) {
			return true
		}

		// Is this an auto decrement?
		if c.t.Peek(i).Is(tokenizer.DecrementToken) {
			return true
		}

		if t.IsReserved(c.flags.extensionsEnabled) {
			return false
		}

		if tokenizer.InList(t,
			tokenizer.BlockBeginToken,
			tokenizer.SemicolonToken,
			tokenizer.EndOfTokens) {
			return false
		}
	}

	return false
}

// Check to see if this is a list of lvalues, which can occur
// in a multi-part assignment.
func assignmentTargetList(c *Compiler) (*bytecode.ByteCode, error) {
	bc := bytecode.New("lvalue list")
	count := 0
	names := []string{}

	savedPosition := c.t.TokenP
	isLvalueList := false

	bc.Emit(bytecode.StackCheck, 1)

	if c.t.Peek(1).Is(tokenizer.PointerToken) {
		return nil, c.compileError(errors.ErrInvalidSymbolName, "*")
	}

	for {
		name := c.t.Next()
		if !name.IsIdentifier() {
			c.t.Set(savedPosition)

			return nil, c.compileError(errors.ErrInvalidSymbolName, name)
		}

		name = tokenizer.NewIdentifierToken(c.normalize(name.Spelling()))
		needLoad := true

		// Until we get to the end of the lvalue...
		for tokenizer.InList(c.t.Peek(1), tokenizer.DotToken, tokenizer.StartOfArrayToken) {
			if needLoad {
				if err := c.ReferenceSymbol(name.Spelling()); err != nil {
					return nil, err
				}

				bc.Emit(bytecode.Load, name)

				needLoad = false
			}

			if err := c.lvalueTerm(bc); err != nil {
				return nil, err
			}
		}

		// Cheating here a bit; this opcode does an optional create
		// if it's not found anywhere in the tree already.
		bc.Emit(bytecode.SymbolOptCreate, name)
		c.ReferenceOrDefineSymbol(name.Spelling())

		names = append(names, name.Spelling())
		patchStore(bc, name.Spelling(), false, false)

		count++

		if c.t.Peek(1).Is(tokenizer.CommaToken) {
			c.t.Advance(1)

			isLvalueList = true

			continue
		}

		if tokenizer.InList(c.t.Peek(1),
			tokenizer.AssignToken,
			tokenizer.DefineToken,
			tokenizer.ChannelReceiveToken) {
			break
		}
	}

	if isLvalueList {
		// If this is a channel store, then a list is not supported yet.
		if c.t.Peek(1).Is(tokenizer.ChannelReceiveToken) {
			return nil, c.compileError(errors.ErrInvalidChannelList)
		}

		// Patch up the stack size check. We can use the SetAddress
		// operator to do this because it really just updates the
		// integer instruction argument.
		_ = bc.SetAddress(0, count)

		// Also, add an instruction that will drop the marker value
		bc.Emit(bytecode.DropToMarker)

		for _, name := range names {
			if err := c.ReferenceSymbol(name); err != nil {
				return nil, err
			}
		}

		return bc, nil
	}

	c.t.TokenP = savedPosition

	return nil, c.compileError(errors.ErrNotAnLValueList)
}

// assignmentTarget compiles the information on the left side of
// an assignment. This information is used later to store the
// data in the named object.
func (c *Compiler) assignmentTarget() (*bytecode.ByteCode, error) {
	if bc, err := assignmentTargetList(c); err == nil {
		return bc, nil
	}

	// Add a marker in the regular code stream here
	c.b.Emit(bytecode.Push, bytecode.NewStackMarker("let"))

	bc := bytecode.New("lvalue")
	isPointer := false

	// Let's look at the first token. This tells us if it is a direct
	// store versus a pointer store.
	name := c.t.Next()

	// If it's a pointer as the first token, this is a pointer store
	// through an address. Use the standard expression evaluator to
	// generate code that gets the pointer value, and then add the
	// StoreViaPointer with no operand, which mean suse the top-of-stack
	// as the address (the TOS must be a pointer type or an error occurs).
	if name.Is(tokenizer.PointerToken) {
		lv, err := c.Expression(true)
		if err != nil {
			return nil, err
		}

		bc.Append(lv)
		bc.Emit(bytecode.StoreViaPointer)

		return bc, nil
	}

	// Not a pointer operation, so we require it to be a valid identifier.
	if !name.IsIdentifier() {
		return nil, c.compileError(errors.ErrInvalidSymbolName, name)
	}

	name = c.normalizeToken(name)
	needLoad := true

	// Until we get to the end of the lvalue...
	for c.t.Peek(1).Is(tokenizer.DotToken) || c.t.Peek(1).Is(tokenizer.StartOfArrayToken) {
		if needLoad {
			if err := c.ReferenceSymbol(name.Spelling()); err != nil {
				return nil, err
			}

			bc.Emit(bytecode.Load, name)

			needLoad = false
		}

		if err := c.lvalueTerm(bc); err != nil {
			return nil, err
		}
	}

	// Quick optimization; if the name is "_" it just means
	// discard and we can short-circuit that.
	if name.Spelling() == defs.DiscardedVariable {
		bc.Emit(bytecode.Drop, 1)
	} else {
		// If its the case of x := <-c  then skip the assignment
		if tokenizer.InList(c.t.Peek(1), tokenizer.AssignToken, tokenizer.DefineToken) && c.t.Peek(2).Is(tokenizer.ChannelReceiveToken) {
			c.t.Advance(1)
		}

		if c.t.Peek(1).Is(tokenizer.DefineToken) {
			bc.Emit(bytecode.SymbolCreate, name)
			c.DefineSymbol(name.Spelling())
		}

		patchStore(bc, name.Spelling(), isPointer, c.t.Peek(1).Is(tokenizer.ChannelReceiveToken))
	}

	bc.Emit(bytecode.DropToMarker, bytecode.NewStackMarker("let"))
	bc.Seal()

	return bc, nil
}

// Helper function for LValue processing. If the token stream we are
// generating ends in a LoadIndex, but this is the last part of the
// storage bytecode, convert the last operation to a Store which writes
// the value back.
func patchStore(bc *bytecode.ByteCode, name string, isPointer, isChan bool) {
	address := bc.Mark() - 1
	instruction := bc.Instruction(address)

	if address > 0 && instruction.Operation == bytecode.LoadIndex && instruction.Operand == nil {
		bc.EmitAt(address, bytecode.StoreIndex)
	} else {
		if isChan {
			bc.Emit(bytecode.StoreChan, name)
		} else {
			if isPointer {
				bc.Emit(bytecode.StoreViaPointer, name)
			} else {
				bc.Emit(bytecode.Store, name)
			}
		}
	}
}

// lvalueTerm parses secondary lvalue operations (array indexes, or struct member dereferences).
func (c *Compiler) lvalueTerm(bc *bytecode.ByteCode) error {
	term := c.t.Peek(1)
	if term.Is(tokenizer.StartOfArrayToken) {
		c.t.Advance(1)

		expression, err := c.Expression(true)
		if err != nil {
			return err
		}

		bc.Append(expression)

		if !c.t.IsNext(tokenizer.EndOfArrayToken) {
			return c.compileError(errors.ErrMissingBracket)
		}

		bc.Emit(bytecode.LoadIndex)

		return nil
	}

	if term.Is(tokenizer.DotToken) {
		c.t.Advance(1)

		member := c.t.Next()
		if !member.IsIdentifier() {
			return c.compileError(errors.ErrInvalidSymbolName, member)
		}

		// Must do this as a push/loadindex in case the struct is
		// actually a typed struct.
		bc.Emit(bytecode.Push, c.normalize(member.Spelling()))
		bc.Emit(bytecode.LoadIndex)

		return nil
	}

	return nil
}
