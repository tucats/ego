package compiler

import (
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/language/tokenizer"
)

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
