package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// isAssignmentTarget peeks ahead in the token stream to determine whether the
// current position looks like the left-hand side of an assignment statement.
// This is used in ambiguous situations — for example, inside an "if" or "for"
// preamble where the compiler cannot tell yet whether it is looking at an
// initialiser assignment or a plain expression.
//
// The check is heuristic: it saves the current token position, scans up to
// 100 tokens forward looking for one of the assignment operators (:=, =, <-,
// +=, etc.) or an auto-increment/decrement token. If it finds one before
// hitting a block boundary or end-of-tokens, it returns true. The token
// position is always restored before returning.
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

// assignmentTargetList attempts to compile a comma-separated list of
// assignment targets for a multi-value assignment such as:
//
//	a, b = someFunc()
//	x, y, z := 1, 2, 3
//
// If only a single name is found before an assignment operator, the function
// returns ErrNotAnLValueList and the caller falls back to the single-target
// path. When a genuine list is detected, a StackCheck instruction is emitted
// first to verify that the right-hand side pushed exactly as many values as
// there are targets, followed by individual Store instructions for each name.
// A DropToMarker instruction at the end discards the stack marker.
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

// assignmentTarget compiles the left-hand side of an assignment into a
// separate bytecode buffer (not the main stream). The returned bytecode,
// when appended to the main stream after the right-hand side expression,
// stores the evaluated value into the correct memory location.
//
// Three lvalue forms are handled:
//
//  1. Multi-target list (a, b = …): delegated to assignmentTargetList.
//
//  2. Pointer dereference (*ptr = …): the expression is compiled via
//     Expression() and a StoreViaPointer instruction is appended.
//
//  3. Simple name with optional suffixes (a.field[i] = …): the base name
//     is parsed, followed by zero or more ".member" or "[index]" suffixes
//     compiled by lvalueTerm. The last LoadIndex (if present) is converted
//     to a StoreIndex by patchStore; otherwise a plain Store is emitted.
//
// A stack marker ("let") is pushed in the main bytecode before this function
// is called so that DropToMarker at the end of the returned buffer can clean
// up any intermediate values left on the stack.
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

// patchStore finalises the store operation at the end of an lvalue bytecode
// buffer. When the last emitted instruction is a LoadIndex with no operand —
// meaning the previous suffix was an array/map subscript — it is replaced
// in-place with StoreIndex, which writes the value back to the element.
// For all other cases a new instruction is appended:
//   - StoreChan  if isChan is true  (channel send: ch <- value)
//   - StoreViaPointer if isPointer is true  (pointer write: *p = value)
//   - Store otherwise  (ordinary variable write)
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

// lvalueTerm compiles a single suffix of a chained lvalue — either an array
// subscript ("[expr]") or a struct/map member access (".name"). The generated
// instructions are appended to the supplied bytecode buffer rather than the
// main stream, because the entire lvalue chain is built separately and later
// appended in the right position relative to the right-hand-side expression.
//
// For "[expr]": the index expression is compiled and a LoadIndex instruction
// appended. LoadIndex will later be patched to StoreIndex by patchStore when
// this is the last suffix in the chain.
//
// For ".name": the member name is pushed as a string constant followed by
// a LoadIndex instruction. Using Push+LoadIndex (rather than the Member
// instruction used in read expressions) ensures that typed struct field
// writes go through the same index-based dispatch path.
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
