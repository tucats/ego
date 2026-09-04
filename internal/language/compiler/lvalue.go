package compiler

import (
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// isAssignmentTarget peeks ahead in the token stream to determine whether the
// current position looks like the left-hand side of an assignment statement.
// This is used in ambiguous situations — for example, inside an "if" or "for"
// preamble where the compiler cannot tell yet whether it is looking at an
// initializer assignment or a plain expression.
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

// multiTargetIsDeclaration performs a bounded, non-consuming lookahead from
// the current token position to decide whether an upcoming comma-separated
// assignment target list is a declaration ("a, b := ...") or a plain
// assignment ("a, b = ..."). It tracks "[...]" and "(...)" nesting so a
// compound target's index/call expression (e.g. "m[f(x)], y = ...") can't be
// mistaken for the list's own terminating operator. The scan stops -- and
// this reports "not a declaration" -- at a block boundary, semicolon, or end
// of input reached before any operator is found; that only happens when the
// list turns out not to be a valid lvalue list at all, in which case the
// caller (assignmentTargetList) errors out or falls back before this answer
// is ever used.
func (c *Compiler) multiTargetIsDeclaration() bool {
	depth := 0

	for i := 1; i < 200; i++ {
		t := c.t.Peek(i)

		switch {
		case tokenizer.InList(t, tokenizer.StartOfArrayToken, tokenizer.StartOfListToken):
			depth++

		case tokenizer.InList(t, tokenizer.EndOfArrayToken, tokenizer.EndOfListToken):
			depth--

		case depth == 0 && t.Is(tokenizer.DefineToken):
			return true

		case depth == 0 && tokenizer.InList(t, tokenizer.AssignToken, tokenizer.ChannelReceiveToken):
			return false

		case depth == 0 && tokenizer.InList(t, tokenizer.BlockBeginToken, tokenizer.SemicolonToken, tokenizer.EndOfTokens):
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

	// A comma-separated target list is used for both "a, b := ..." (declare,
	// creating new locals -- shadowing is intentional here, exactly like a
	// single-target ":=") and "a, b = ..." (assign, which must find and
	// update whatever "a" and "b" already resolve to, possibly in an outer
	// scope). The operator that decides which of those this is comes AFTER
	// the whole name list in the token stream, but SymbolOptCreate/Store
	// bytecode for each name is emitted DURING the loop below, one name at a
	// time, before that operator is seen. So the operator is found here via a
	// bounded, non-consuming lookahead first.
	isDeclaration := c.multiTargetIsDeclaration()

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

		// Reject shadowing a built-in type name when
		// ego.compiler.type.shadowing is turned off (BUG-75). Restore the
		// tokenizer position first, matching every other early-return in
		// this loop: assignmentTarget's caller silently falls back to the
		// single-target lvalue path on ANY error from this function (it
		// only distinguishes "not a list" from "real error" by discarding
		// errors wholesale), so leaving the tokenizer mid-token here would
		// make that fallback re-parse from the wrong position.
		if err := c.checkTypeShadowing(name.Spelling()); err != nil {
			c.t.Set(savedPosition)

			return nil, err
		}

		// Until we get to the end of the lvalue...
		for tokenizer.InList(c.t.Peek(1), tokenizer.DotToken, tokenizer.StartOfArrayToken) {
			if needLoad {
				if err := c.ReferenceSymbol(name.Spelling()); err != nil {
					return nil, err
				}

				c.emitLoadName(bc, name.Spelling())

				needLoad = false
			}

			if err := c.lvalueTerm(bc); err != nil {
				return nil, err
			}
		}

		// Cheating here a bit; this opcode does an optional create if it's
		// not found in the CURRENT scope already (SymbolOptCreate's runtime
		// processor, symbolCreateIfByteCode, checks GetLocal -- current table
		// only, exactly like a plain ":=" declaration does, so a name already
		// bound in an outer scope is correctly shadowed by a new local here,
		// not overwritten). This only applies to a SIMPLE lvalue: needLoad is
		// still true here exactly when the suffix loop above never ran, i.e.
		// there was no ".field"/"[index]" chain. A compound target's base
		// variable must already exist (you cannot introduce "m" via
		// "m[\"k\"] := 5") and was already Load'ed and ReferenceSymbol'd
		// inside that loop, so nothing further is needed for it here --
		// emitting SymbolOptCreate unconditionally for every target, compound
		// or not, used to corrupt the very next patchStore call below.
		// patchStore decides whether to convert the lvalue chain's trailing
		// LoadIndex into StoreIndex by checking whether the LAST instruction
		// emitted so far is exactly that LoadIndex; SymbolOptCreate landing
		// in between made that check fail, silently falling back to an
		// ordinary Store on the base variable name instead of storing into
		// the map/array/struct element at all (BUG-24).
		//
		// A SIMPLE target of a plain "=" list (not ":="), by contrast, must
		// NOT go through SymbolOptCreate at all: since it is current-scope-
		// only, emitting it here for "predigit, nines = 0, 0" inside a block
		// nested below wherever "predigit"/"nines" were declared silently
		// created a fresh local shadow in the current (inner) block scope
		// instead of updating the outer variable -- the write vanished the
		// moment that inner scope was popped, indistinguishable from the
		// assignment having no effect at all. The single-target path
		// (assignmentTarget, below) already gets this right: it only emits a
		// create opcode when the operator is ":=" (see its own
		// "tokenizer.DefineToken" check); for "=" it just emits Store, which
		// searches the full scope chain via c.set the same way a symbol
		// lookup does. This mirrors that here for the *opcode*, gated on
		// isDeclaration (computed once above before this loop starts, since
		// the operator itself appears only after the whole comma-separated
		// name list).
		//
		// The ReferenceOrDefineSymbol call, though, stays unconditional even
		// for "=": assignmentTargetList is tried speculatively for EVERY
		// assignment, including plain single-name ones like "err = e" that
		// turn out not to be a list at all once the whole statement is seen
		// (no comma). Its side effect of marking "err" as used in the
		// unused-variable tracker is, in that case, the ONLY thing that
		// prevents an earlier "err := nil" from being flagged as unused --
		// gating this call on isDeclaration too (an earlier version of this
		// fix did) silently broke that for every plain "=" reassignment of a
		// declared-but-not-yet-read variable. mustExist stays false
		// (ReferenceOrDefineSymbol, not ReferenceSymbol) so this speculative,
		// possibly-not-really-a-list call can never itself raise a "not
		// found" compile error the way the real single- or multi-target
		// path's own checks would.
		if needLoad {
			if isDeclaration {
				bc.Emit(bytecode.SymbolOptCreate, name)
			}

			c.ReferenceOrDefineSymbol(name.Spelling())
		}

		names = append(names, name.Spelling())
		c.patchStore(bc, name.Spelling(), false, false, -1)

		// A compound target's StoreIndex (emitted by patchStore just above)
		// pushes its container back onto the stack on success -- storeInMap,
		// storeInArray, and the *data.Struct/*any-wrapped-struct branches of
		// storeIndexByteCode (internal/language/bytecode/structs.go) all do
		// this unconditionally. For a single-target assignment that leftover
		// is harmless: the "let" marker pushed before the lvalue runs, and
		// the DropToMarker at the very end of that path, cleans it up in one
		// shot regardless of how many stray items accumulated. But within
		// THIS list, the next target's value must be immediately below
		// where this target's Load/suffix chain started -- so the leftover
		// container has to be discarded right now, or it silently becomes
		// the next target's "value" instead of the real one (e.g.
		// "m[\"k\"], arr[0] = pair()" stored the whole map "m" into
		// arr[0], not pair()'s second return value) (BUG-24).
		if !needLoad {
			bc.Emit(bytecode.Drop, 1)
		}

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

			c.emitLoadName(bc, name.Spelling())

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
		declSlot := -1

		if c.t.Peek(1).Is(tokenizer.DefineToken) {
			// Reject shadowing a built-in type name when
			// ego.compiler.type.shadowing is turned off (BUG-75).
			if err := c.checkTypeShadowing(name.Spelling()); err != nil {
				return nil, err
			}

			// docs/SLOTS.md: in a slot-eligible function, a simple ":=" of a
			// slot-eligible name is given a compile-time slot instead of a
			// runtime symbol-table entry. There is no separate "create" step -
			// the slot already exists in the bank (AllocateLocal) - so no
			// SymbolCreate is emitted; the reserved index is threaded to
			// patchStore below, which emits the StoreSlot. Registration of the
			// name is deferred (allocateSlot records it as pending) until the
			// enclosing statement's RHS has been compiled, so a shadowing
			// "x := x + 1" still reads the outer x.
			if idx, ok := c.allocateRegister(name.Spelling()); ok {
				declSlot = idx
			} else if c.inIdempotentDeclScope() {
				// PERFORMANCE.md Finding 11: inside a for-loop body scope that
				// compileForBody has proven safe for it (see
				// loopBodyIdempotentDeclEligible), a simple ":=" must not error
				// when the name already exists - it means this is the second or
				// later iteration reusing the loop's single shared scope, not a
				// genuine duplicate declaration.
				bc.Emit(bytecode.SymbolOptCreate, name)

				c.nonConstLocalNames[name.Spelling()] = true
			} else {
				bc.Emit(bytecode.SymbolCreate, name)
				
				c.nonConstLocalNames[name.Spelling()] = true
			}

			c.DefineSymbol(name.Spelling())
		}

		// isChan (channel SEND, "ch <- value") is true here exactly when
		// "<-" is the very next token, which only happens when "<-" is
		// itself playing the role of the assignment operator (there is no
		// ":="/"=" in a send statement at all). A single-value channel
		// RECEIVE ("x := <-c") used to be detected here too, via a small
		// hack that peeked past the still-unconsumed ":="/"=" to see the
		// "<-" beyond it and skip past the operator early -- but doing so
		// made this lvalue's store code hard-wire the receive (StoreChan)
		// into the assignment itself, which only worked when "<-ch" was the
		// *entire* right-hand side ("x := <-ch + 1" broke, since the
		// receive can't be separated from the "+ 1" that way). Now that
		// "<-" is a general expression atom (expressionAtom in
		// expr_atom.go, BUG-62/BUG-72), the receive is compiled as part of
		// the ordinary right-hand-side expression instead, which already
		// leaves a plain received value on the stack -- so this lvalue no
		// longer needs to special-case receives at all, only sends.
		c.patchStore(bc, name.Spelling(), isPointer, c.t.Peek(1).Is(tokenizer.ChannelReceiveToken), declSlot)
	}

	bc.Emit(bytecode.DropToMarker, bytecode.NewStackMarker("let"))
	bc.Seal()

	return bc, nil
}

// patchStore finalizes the store operation at the end of an lvalue bytecode
// buffer. When the last emitted instruction is a LoadIndex with no operand —
// meaning the previous suffix was an array/map/struct-field subscript — it
// is replaced in-place with one of:
//   - StoreIndexChan if isChan is true — a channel SEND through a compound
//     lvalue, e.g. "s.ch <- value" or "chans[0] <- value" (BUG-73). This
//     reads back whatever is currently stored at that index/key, requires
//     it to already be a channel, and sends to it — it does NOT overwrite
//     the field/element, unlike an ordinary StoreIndex.
//   - StoreIndex otherwise — writes the value back to the element.
//
// For all other (non-compound) lvalues a new instruction is appended:
//   - StoreChan  if isChan is true  (channel send: ch <- value)
//   - StoreViaPointer if isPointer is true  (pointer write: *p = value)
//   - Store otherwise  (ordinary variable write)
//
// declSlot is the slot index reserved for a slotted ":=" declaration of name,
// or -1 when this store is not such a declaration. When declSlot >= 0 the store
// is emitted as StoreSlot with that index directly (the binding is not yet
// name-resolvable - see allocateSlot's deferred registration). Otherwise, for
// the ordinary variable-store case, emitStoreName resolves name to a StoreSlot
// when it is an already-declared slotted local and a name-based Store when it is
// not (a parameter, global, or list-declared name).
func (c *Compiler) patchStore(bc *bytecode.ByteCode, name string, isPointer, isChan bool, declSlot int) {
	address := bc.Mark() - 1
	instruction := bc.Instruction(address)

	if address > 0 && instruction.Operation == bytecode.LoadIndex && instruction.Operand == nil {
		if isChan {
			bc.EmitAt(address, bytecode.StoreIndexChan)
		} else {
			bc.EmitAt(address, bytecode.StoreIndex)
		}
	} else {
		if isChan {
			bc.Emit(bytecode.StoreChan, name)
		} else if isPointer {
			bc.Emit(bytecode.StoreViaPointer, name)
		} else if declSlot >= 0 {
			bc.Emit(bytecode.StoreRegister, declSlot)
		} else {
			c.emitStoreName(bc, name)
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
