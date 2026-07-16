package compiler

import (
	"strings"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// slotContext holds the per-function state for compile-time slot assignment
// (docs/SLOTS.md). One is created in generateFunctionBytecode for each function
// proven slot-eligible, and it lives on the compiler that compiles that
// function's body. Nested functions get their own context (generateFunctionBytecode
// resets it after Clone), so slot numbering never leaks across function boundaries.
type slotContext struct {
	// names maps each assigned slot index to the source name declared there.
	// Its length is the total number of slots -- the operand patched into the
	// function's AllocateLocal instruction -- and it doubles as the debug
	// name/slot table consumed by introspection (Q3). A name that is shadowed
	// in a nested block appears more than once, at different indices, which is
	// correct: each distinct lexical binding gets its own slot.
	names []string

	// allocateAddr is the bytecode address of the placeholder instruction
	// (emitted as NoOperation right after the function's boundary PushScope)
	// that is patched into "AllocateLocal <len(names)>" once the whole body has
	// been compiled and the final slot count is known.
	allocateAddr int

	// scopeStart is the c.scopes index at or above which this function's own
	// slot bindings live. resolveSlot never looks below it, so an enclosing
	// slot-eligible function's bindings (shared into c.scopes by Clone) are
	// never mistaken for this function's.
	scopeStart int

	// pending holds slot bindings whose index has been allocated (and whose
	// StoreSlot has been emitted) but whose name->slot registration in the
	// lexical scope is deferred until the declaring statement's right-hand side
	// has finished compiling. This reproduces Go's rule that the new variable
	// in "x := <rhs>" is not in scope while <rhs> is evaluated -- so a
	// shadowing "x := x + 1" resolves the RHS "x" to the OUTER binding. See the
	// KEY DESIGN FINDING note in docs/SLOTS.md.
	pending []pendingSlotDecl
}

// pendingSlotDecl is one deferred slot registration (see slotContext.pending).
type pendingSlotDecl struct {
	name     string
	idx      int
	scopePos int
}

// slotsActive reports whether the compiler is currently emitting code for a
// slot-eligible function body.
func (c *Compiler) slotsActive() bool {
	return c.funcSlots != nil
}

// isRangeLoopAhead reports whether the tokenizer, positioned at the target of a
// for statement's clause, is looking at a range loop ("for <targets> := range
// x") rather than an iteration loop ("for i := 0; ..."). It is a read-only
// Peek-only lookahead - it never advances the tokenizer - that scans past the
// comma-separated target identifiers and the ":="/"=" operator and checks for a
// following "range". Used to decide whether the loop's index/value targets must
// stay name-based (range) or may be slotted (iteration); see the call site in
// the for-loop compiler and docs/SLOTS.md.
func (c *Compiler) isRangeLoopAhead() bool {
	p := 1
	for c.t.Peek(p).IsIdentifier() || c.t.Peek(p).Is(tokenizer.CommaToken) {
		p++
	}

	if c.t.Peek(p).Is(tokenizer.DefineToken) || c.t.Peek(p).Is(tokenizer.AssignToken) {
		return c.t.Peek(p + 1).Is(tokenizer.RangeToken)
	}

	return false
}

// slotEligibleName reports whether name is one that slot assignment should
// handle at all: a real user binding, not the discard "_", a generated "$"
// temporary, or a "_"-prefixed readonly constant (left name-based in this cut
// so the readonly machinery in the name-based opcodes is unchanged).
func slotEligibleName(name string) bool {
	if name == "" || name == defs.DiscardedVariable || strings.HasPrefix(name, "$") {
		return false
	}

	if strings.HasPrefix(name, defs.ReadonlyVariablePrefix) {
		return false
	}

	return true
}

// allocateSlot reserves the next slot index for a ":="-declared name and
// records a pending (deferred) registration for it, but does NOT yet make the
// name resolvable -- registerPendingSlots does that once the declaring
// statement's RHS is compiled. It returns the reserved index (for the StoreSlot
// operand) and true, or (-1, false) when slots are inactive, suppressed, or
// name is not slot-eligible, in which case the caller keeps the name-based
// declaration.
func (c *Compiler) allocateSlot(name string) (int, bool) {
	if c.funcSlots == nil || c.flags.suppressSlotDecl || !slotEligibleName(name) {
		return -1, false
	}

	idx := len(c.funcSlots.names)
	c.funcSlots.names = append(c.funcSlots.names, name)

	c.funcSlots.pending = append(c.funcSlots.pending, pendingSlotDecl{
		name:     name,
		idx:      idx,
		scopePos: len(c.scopes) - 1,
	})

	return idx, true
}

// registerPendingSlots makes every slot reserved since the last call resolvable
// by name, binding each into the lexical scope it was declared in. It is called
// once the current declaring statement's RHS has been fully compiled (see the
// deferred call in compileAssignment and the for-loop init flush), so an RHS
// never sees a binding the same statement is introducing.
func (c *Compiler) registerPendingSlots() {
	if c.funcSlots == nil || len(c.funcSlots.pending) == 0 {
		return
	}

	for _, p := range c.funcSlots.pending {
		if p.scopePos < 0 || p.scopePos >= len(c.scopes) {
			continue
		}

		if c.scopes[p.scopePos].slots == nil {
			c.scopes[p.scopePos].slots = map[string]int{}
		}

		c.scopes[p.scopePos].slots[p.name] = p.idx
	}

	c.funcSlots.pending = c.funcSlots.pending[:0]
}

// resolveSlot returns the slot index bound to name in the current function, and
// true, if name resolves to one of this function's own slots. It walks the
// lexical scope stack from innermost outward, stopping at the function's
// scopeStart so that an enclosing function's slots are never returned. When
// slots are inactive, or name is not a slotted local (a parameter, global,
// package member, or generated temporary), it returns (-1, false) and the
// caller falls back to the name-based opcode.
func (c *Compiler) resolveSlot(name string) (int, bool) {
	if c.funcSlots == nil {
		return -1, false
	}

	for i := len(c.scopes) - 1; i >= c.funcSlots.scopeStart && i >= 0; i-- {
		if c.scopes[i].slots != nil {
			if idx, ok := c.scopes[i].slots[name]; ok {
				return idx, true
			}
		}
	}

	return -1, false
}

// emitLoadName emits a load of name into buffer b, choosing LoadSlot when name
// resolves to one of the current function's slots and the name-based Load
// otherwise. This is the single choke point every identifier-read site routes
// through, so a non-slotted name (parameter, global, "$"-temporary)
// transparently keeps its existing behavior.
func (c *Compiler) emitLoadName(b *bytecode.ByteCode, name string) {
	if idx, ok := c.resolveSlot(name); ok {
		b.Emit(bytecode.LoadSlot, idx)
	} else {
		b.Emit(bytecode.Load, name)
	}
}

// emitAddressOfName emits an address-of for name into buffer b, choosing
// AddressOfSlot for a slotted local and AddressOf otherwise.
func (c *Compiler) emitAddressOfName(b *bytecode.ByteCode, name string) {
	if idx, ok := c.resolveSlot(name); ok {
		b.Emit(bytecode.AddressOfSlot, idx)
	} else {
		b.Emit(bytecode.AddressOf, name)
	}
}

// emitStoreName emits an ordinary variable store (the value is on top of the
// stack) into buffer b, choosing StoreSlot for a slotted local and Store
// otherwise. This is the write counterpart of emitLoadName.
func (c *Compiler) emitStoreName(b *bytecode.ByteCode, name string) {
	if idx, ok := c.resolveSlot(name); ok {
		b.Emit(bytecode.StoreSlot, idx)
	} else {
		b.Emit(bytecode.Store, name)
	}
}

// emitDeRefName emits a pointer dereference of name into buffer b. For a
// name-based local the single DeRef opcode does the symbol lookup and
// dereference. For a slotted local, whose value the DeRef opcode could not find
// by name, the pointer value is first loaded from its slot into a fresh
// temporary and DeRef is applied to that temporary -- the same store-in-temp /
// DeRef-temp idiom compilePointerDereference already uses for dereferencing an
// arbitrary expression.
func (c *Compiler) emitDeRefName(b *bytecode.ByteCode, name string) {
	idx, ok := c.resolveSlot(name)
	if !ok {
		b.Emit(bytecode.DeRef, name)

		return
	}

	temp := data.GenerateName()
	b.Emit(bytecode.LoadSlot, idx)
	b.Emit(bytecode.StoreAlways, temp)
	b.Emit(bytecode.DeRef, temp)
}

// This file implements the compile-time analysis behind slot-based local
// variable access (see docs/SLOTS.md). Phase 1 uses a single, deliberately
// conservative token-level predicate -- in the same style as
// blockBodyNeedsOwnScope / loopBodyNeedsFreshScopePerIteration -- to decide
// whether a function body may have its parameters and locals resolved to
// integer slots at compile time instead of by runtime name lookup.
//
// Being overly cautious here only ever costs a missed optimization, never
// correctness, so every uncertain case takes the "not eligible" answer and
// falls back to the existing, unchanged name-based path.

// functionBodyIsSlotEligible reports whether the function body positioned at
// the current tokenizer mark (the body's opening "{" or the combined "{}"
// empty-block token) is eligible for compile-time slot assignment. seedNames
// must contain the function's own parameter, named-return, and receiver names
// -- the declared names that live outside the body braces -- so the closure
// capture check below can recognize a closure that references one of them.
//
// A function body is slot-eligible only if a one-time scan of its own body
// (nested function bodies are compiled and judged independently) finds:
//
//   - no "go" or "defer" statement at the enclosing function's own level
//     (one inside a nested function literal belongs to that literal, not to
//     this function, so it is ignored here); and
//
//   - no function literal ("func ...") whose body references any name declared
//     in this function (docs/SLOTS.md Section 11, Q2). A literal that captures
//     nothing from the enclosing function -- e.g. a sort.Slice comparator that
//     uses only its own parameters -- does NOT disqualify. This is the refined
//     closure rule: it disqualifies on actual capture rather than on the mere
//     presence of any literal, while still never attempting the harder upvalue
//     problem the design defers.
//
// The capture check is intentionally conservative in the "capturing" direction:
// a literal parameter or local that merely happens to share a name with an
// enclosing declaration is treated as a capture (disqualifying), because
// distinguishing the two would require full lexical resolution. Over-reporting
// capture only forgoes an optimization; under-reporting it would be unsound, so
// the scan is structured to never under-report.
func (c *Compiler) functionBodyIsSlotEligible(seedNames map[string]bool) bool {
	pos := c.t.Mark()
	if pos >= len(c.t.Tokens) {
		return false
	}

	// An empty body declares nothing and contains no literal, so it is trivially
	// eligible (the AllocateLocal it emits will just size the bank to its
	// parameters/returns).
	if c.t.Tokens[pos].Is(tokenizer.EmptyBlockToken) {
		return true
	}

	if !c.t.Tokens[pos].Is(tokenizer.BlockBeginToken) {
		// Not actually a block - malformed input the caller's own validation
		// will report. Take the safe (ineligible) path.
		return false
	}

	// declaredNames accumulates every name this function declares that a nested
	// literal could capture: the seed set (params/returns/receiver) plus every
	// ":="/"var" name found at this function's own level during the scan below.
	declaredNames := map[string]bool{}
	for name := range seedNames {
		declaredNames[name] = true
	}

	// closureRefs accumulates every identifier that appears inside any nested
	// function literal's body. After the scan, a non-empty intersection with
	// declaredNames means some literal captured an enclosing name.
	closureRefs := map[string]bool{}

	depth := 0

	for i := pos; i < len(c.t.Tokens); i++ {
		tok := c.t.Tokens[i]

		switch {
		case tok.Is(tokenizer.FuncToken):
			// A nested function literal (or nested named function). Locate its
			// body and record every identifier it references, then skip the main
			// scan past it so its own declarations never enter declaredNames and
			// its own go/defer never disqualify this function.
			end, ok := c.scanClosureBody(i, closureRefs)
			if !ok {
				// Could not confidently delimit the literal's body (e.g. an
				// inline struct/interface type in its signature). Disqualify
				// rather than risk missing a capture.
				return false
			}

			i = end

		case tok.Is(tokenizer.GoToken), tok.Is(tokenizer.DeferToken):
			return false

		case tok.Is(tokenizer.ChanToken), tok.Is(tokenizer.ChannelReceiveToken):
			// Channel operations (send/receive, and channel-typed locals) go
			// through the name-based StoreChan/ReceiveChannel opcodes, which
			// this first cut does not give slot equivalents. Disqualify rather
			// than risk a slotted channel local reaching a name-based op. Both
			// tokens are unambiguous (unlike "*"/"&", which collide with
			// multiply/bit-and), so a token scan can gate on them safely.
			return false

		case tok.Is(tokenizer.BlockBeginToken):
			depth++

		case tok.Is(tokenizer.BlockEndToken):
			depth--

			// depth == 0 means this closed the body's own opening brace (the
			// scan starts AT that brace, which incremented depth to 1). The
			// whole body has been scanned with nothing disqualifying found; the
			// final answer is whether any literal captured an enclosing name.
			if depth == 0 {
				return !intersects(closureRefs, declaredNames)
			}

		case tok.Is(tokenizer.DefineToken):
			collectDefineTargets(c.t.Tokens, pos, i, declaredNames)

		case tok.Is(tokenizer.VarToken):
			collectVarTargets(c.t.Tokens, i, declaredNames)
		}
	}

	// Unbalanced braces: malformed input the tokenizer would already reject
	// elsewhere. Take the safe path.
	return false
}

// scanClosureBody handles a nested "func" literal starting at index funcPos. It
// records every identifier appearing in the literal's body into refs and
// returns the index of the literal's closing body brace (so the caller can
// resume the outer scan just past it) together with true. It returns ok=false
// when the literal's body cannot be confidently located -- specifically when
// the signature between "func" and the body contains an inline brace-delimited
// type (struct/interface literal), which would make the first "{" ambiguous.
func (c *Compiler) scanClosureBody(funcPos int, refs map[string]bool) (int, bool) {
	tokens := c.t.Tokens

	// Walk the signature (params and return types) to the body's opening brace.
	j := funcPos + 1
	for ; j < len(tokens); j++ {
		t := tokens[j]

		if t.Is(tokenizer.EmptyBlockToken) {
			// "func(...) {}" -- an empty body captures nothing.
			return j, true
		}

		if t.Is(tokenizer.BlockBeginToken) {
			break
		}

		// An inline struct/interface type in the signature would introduce a
		// brace that is not the body brace, defeating the first-"{" heuristic.
		// These are vanishingly rare in a literal's signature; bail safely.
		if t.Is(tokenizer.StructToken) || t.Is(tokenizer.InterfaceToken) {
			return 0, false
		}
	}

	if j >= len(tokens) {
		return 0, false
	}

	// tokens[j] is the body's opening brace. Brace-match to its close, recording
	// every identifier along the way (including those in any further-nested
	// literal, which is exactly what we want -- a capture two levels down is
	// still a capture of this function's name).
	depth := 0

	for k := j; k < len(tokens); k++ {
		t := tokens[k]

		switch {
		case t.Is(tokenizer.BlockBeginToken):
			depth++

		case t.Is(tokenizer.BlockEndToken):
			depth--
			if depth == 0 {
				return k, true
			}

		default:
			if t.IsIdentifier() {
				refs[t.Spelling()] = true
			}
		}
	}

	// Ran off the end without closing the body: malformed. Signal ineligible.
	return 0, false
}

// collectDefineTargets records the names on the left-hand side of a ":="
// declaration whose ":=" token is at index defPos. It walks backward over the
// comma-separated identifier list that must immediately precede ":=", stopping
// at pos (the body start) or the first non-identifier/non-comma token. The
// discard name "_" is ignored. This mirrors the backward walk in
// loopBodyIdempotentDeclEligible.
func collectDefineTargets(tokens []tokenizer.Token, pos, defPos int, out map[string]bool) {
	for j := defPos - 1; j >= pos; j-- {
		prior := tokens[j]

		if prior.Is(tokenizer.CommaToken) {
			continue
		}

		if prior.IsIdentifier() {
			if name := prior.Spelling(); name != defs.DiscardedVariable {
				out[name] = true
			}

			continue
		}

		break
	}
}

// collectVarTargets records the names declared by a "var" statement whose "var"
// token is at index varPos. It collects the leading comma-separated identifier
// run immediately after "var" (e.g. "var a, b int" -> a, b). The "var ( ... )"
// grouped form is only partially covered -- the first line's names are
// collected and the rest are simply not, which is safe: any name that fails to
// enter declaredNames only relaxes the closure-capture check in the
// conservative (still-sound) direction, since a real capture references the
// name and the miss would at worst forgo an optimization for that one name.
// Over-collection is likewise harmless. The discard name "_" is ignored.
func collectVarTargets(tokens []tokenizer.Token, varPos int, out map[string]bool) {
	expectName := true

	for j := varPos + 1; j < len(tokens); j++ {
		t := tokens[j]

		if expectName {
			if t.IsIdentifier() {
				if name := t.Spelling(); name != defs.DiscardedVariable {
					out[name] = true
				}

				expectName = false

				continue
			}

			return
		}

		if t.Is(tokenizer.CommaToken) {
			expectName = true

			continue
		}

		// Anything else (a type, "=", "(", newline handling, etc.) ends the
		// leading name list.
		return
	}
}

// intersects reports whether any key of a is also a key of b.
func intersects(a, b map[string]bool) bool {
	// Iterate the smaller map for a cheaper check.
	if len(b) < len(a) {
		a, b = b, a
	}

	for k := range a {
		if b[k] {
			return true
		}
	}

	return false
}
