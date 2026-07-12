package compiler

// This file compiles the "@capture" directive:
//
//	@capture <var> := { statements }
//	@capture <var> =  { statements }
//
// After the block runs, <var> holds every character the block's statements
// would otherwise have printed to the console (via fmt.Println, fmt.Printf,
// the "print" language extension, etc.), as one plain string. This is
// mainly meant for use inside "@test"/"@assert" blocks, so a test can check
// exactly what a piece of code prints, not just what it returns.
//
// If you are new to Go: this file's job is only to translate the Ego
// SOURCE TEXT above into a sequence of "bytecode" instructions -- think of
// bytecode as a simple list of very small steps (like "push this value",
// "call this function", "jump to instruction 12") that the Ego runtime
// executes one at a time. This file does not do any of the actual
// capturing itself; that happens later, when the compiled program runs and
// reaches the BeginCapture/EndCapture/SyncOutputWriter instructions this
// file emits. Those three instructions are implemented in
// internal/language/bytecode/capture.go, which has a long comment
// explaining exactly what each one does and why there are three of them.

import (
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// captureDirective compiles "@capture <var> := { ... }" or
// "@capture <var> = { ... }". The "@capture" keyword itself has already
// been consumed by the time this method is called (compileDirective, in
// directives.go, reads the directive name and dispatches here).
//
// # The two forms of "<var>"
//
//   - "@capture x := { ... }" declares a brand new variable named x, the
//     same way an ordinary "x := 5" statement would. It is an error to use
//     ":=" if x already exists in the current scope (exactly the same rule
//     ordinary ":=" already follows).
//   - "@capture x = { ... }" assigns to an EXISTING variable named x. Ego's
//     ordinary "x = 5" doesn't actually check at compile time that x exists
//     (it only fails at runtime if it doesn't) -- this directive is
//     deliberately a little stricter than that, and checks at compile time,
//     because it gives a much clearer error message to a test author who
//     made a typo.
//
// # Discarding the captured text with "_"
//
// Sometimes a block is only being wrapped in "@capture" to keep its
// fmt.Println/fmt.Printf output from cluttering the console (or an "ego
// test" run's log), and nothing in the test actually needs to inspect the
// printed text -- for example, a test that already checks fmt.Printf's
// returned byte count and error value has no further use for the text
// itself. For that case, use Ego's ordinary "discard" name in place of a
// real variable:
//
//	@capture _ := {
//	    n, err := fmt.Printf("hello %d\n", 42)
//	    @assert n == 9
//	}
//
// "_" can be used with either ":=" or "=" here, exactly as it can with an
// ordinary assignment -- both forms behave identically. On success, the
// captured text is thrown away immediately instead of being stored
// anywhere. On error, the captured text is still printed to the console
// with the usual "@capture _:" heading (discarding it on success doesn't
// mean you don't want to SEE it if something went wrong) -- only the
// success-path bookkeeping changes.
//
// # Why the block compiles "inline" instead of in its own mini-compiler
//
// The "@compile { ... }" directive (see directives.go's compileBlockDirective)
// deliberately compiles its block in a completely separate, throwaway
// sub-compiler, so that a broken block can't corrupt the surrounding
// program and none of its variables leak out. "@capture" needs the OPPOSITE
// of that: the block must be able to see and use ordinary outer variables
// (that's the whole point -- it's just an ordinary block of code, wrapped
// so its output gets collected), and <var> itself is declared/assigned in
// the OUTER scope so it's still usable after the block ends. So this
// function calls compileRequiredBlock directly, the same helper used to
// compile the body of "if", "for", and "try" -- NOT the isolated
// sub-compiler machinery "@compile" uses.
//
// # Why this looks so much like "try { } catch(e) { }"
//
// If something inside the block goes wrong (any ordinary catchable runtime
// error -- a division by zero, a failed assertion, and so on), "@capture"
// still needs to:
//  1. stop capturing and put back whatever writer was in use before, so
//     later code doesn't keep writing into an abandoned buffer, and
//  2. make sure the output that WAS captured before the error isn't just
//     silently thrown away -- it's often the most useful clue for figuring
//     out what went wrong.
//
// Ego's "try { } catch(e) { }" statement already solves the general version
// of problem #1 (see compileTry in try.go): if an error happens inside the
// try body, control jumps straight to a catch handler instead of falling
// off the end of the program. So this function generates the exact same
// kind of Try/catch-handler/TryPop skeleton compileTry does, wrapped around
// the block -- except the "catch handler" here is not something the Ego
// program author wrote (there's no visible "catch(e)" in @capture's own
// syntax); it's a small fixed sequence this function generates automatically
// to do the cleanup described above, print the captured text with a
// heading like "@capture output:" so it's not lost, and then RE-RAISE
// (rethrow) the very same error so it keeps propagating outward -- to a
// real, user-written "try/catch" further out if there is one, or all the
// way out to end the program if there isn't. Re-raising an already-caught
// error is exactly what the "throw" statement does (see throw.go), and
// this function uses the very same Throw instruction to do it.
//
// # A known, deliberate limitation
//
// Ego's "@fail" directive and an unrecovered panic() both intentionally
// bypass try/catch entirely elsewhere in this codebase (see catch.go and
// panic.go) -- there is no "catch" clause anywhere that can intercept them,
// by design, because both are meant to be unconditionally fatal. Since
// "@capture" builds on the SAME try/catch machinery, it inherits the same
// limitation: if "@fail" runs, or a panic() inside the block is never
// recovered, the cleanup/flush logic below never runs, and whatever had
// been captured so far is lost. In practice this is an acceptable trade-off
// -- the whole program is already stopping in both of those cases, so
// nothing downstream is left in a confusing half-restored state -- but it
// is worth knowing about if a captured buffer ever seems to have
// "disappeared" after one of those two specific failure modes.
func (c *Compiler) captureDirective() error {
	// Step 1: read the variable name. It must be a plain identifier, e.g.
	// "output" -- not an expression, not a dotted/array reference.
	varToken := c.t.Next()
	if !varToken.IsIdentifier() {
		return c.compileError(errors.ErrInvalidSymbolName, varToken)
	}

	// normalize() applies this compiler's case-folding rules (controlled by
	// the "normalizedIdentifiers" flag) so "@capture Output" and a later
	// "output" reference agree on whether they mean the same variable --
	// the same helper every other identifier-reading directive uses.
	varName := c.normalize(varToken.Spelling())

	// "_" is Ego's ordinary "I don't want this value" name (the same one
	// "_ := someFunc()" already uses everywhere else in the language). When
	// it's used here, the captured text is thrown away on success instead
	// of being declared/stored as a real variable -- see the "Discarding
	// the captured text" section of this function's doc comment above.
	isDiscard := varName == defs.DiscardedVariable

	// Step 2: the variable name must be followed by ":=" (declare) or "="
	// (assign to an existing variable) -- nothing else is valid here.
	isDefine := c.t.IsNext(tokenizer.DefineToken)
	if !isDefine && !c.t.IsNext(tokenizer.AssignToken) {
		return c.compileError(errors.ErrMissingAssignment)
	}

	// Step 3: emit the same "Try" setup compileTry uses (see try.go) -- this
	// arranges for control to jump to the catch-handler code emitted in
	// Step 7 below if anything in the block raises a catchable error. "b1"
	// is a placeholder address that gets patched, once we know where the
	// catch handler actually starts, by SetAddressHere further down.
	b1 := c.b.Mark()
	tryMarker := bytecode.NewStackMarker("try")
	c.b.Emit(bytecode.Try, 0)
	c.b.Emit(bytecode.Push, tryMarker)

	// Step 4: declare or validate <var> now, BEFORE the risky block runs.
	// Doing this up front (rather than separately in the success path and
	// the catch path) means both of those paths can end with a plain
	// "Store" into an already-known-to-exist variable, instead of having to
	// worry about declaring it twice.
	//
	// None of this applies when discarding with "_": Ego never actually
	// declares or stores into "_" anywhere in the language (an ordinary
	// "_ := foo()" statement compiles straight to a Drop instruction, with
	// no SymbolCreate/Store at all -- see lvalue.go). Skipping this step
	// for "_" matters for more than just tidiness: emitting SymbolCreate
	// for "_" here would make a SECOND "@capture _ := { ... }" in the same
	// scope fail at runtime (symbol "_" already exists) -- and because
	// that failure would happen before BeginCapture even runs, the catch
	// handler below would call EndCapture with nothing to match it,
	// corrupting the capture stack instead of cleanly reporting an error.
	if !isDiscard {
		if isDefine {
			// Reject shadowing a built-in type name when
			// ego.compiler.type.shadowing is turned off (BUG-75).
			if err := c.checkTypeShadowing(varName); err != nil {
				return err
			}

			c.b.Emit(bytecode.SymbolCreate, varName)
			c.DefineSymbol(varName)
		} else {
			if err := c.ReferenceSymbol(varName); err != nil {
				return err
			}
		}
	}

	// Step 5: start capturing. From this instruction on, anything the
	// block prints goes into a temporary buffer instead of wherever it was
	// going before (see beginCaptureByteCode in the bytecode package).
	c.b.Emit(bytecode.BeginCapture)

	// Step 6: compile the block's own statements. Passing (false, false)
	// mirrors exactly how compileTry compiles an ordinary try-body: this is
	// not a function body (so no RunDefers on exit), and its own scope
	// should not be skipped/elided.
	if err := c.compileRequiredBlock(false, false); err != nil {
		return err
	}

	// Step 7a: the SUCCESS path -- reached only if the block finished
	// without raising an error. DropToMarker cleans up anything left on the
	// value stack by the block (mirrors compileTry). Then: stop capturing,
	// save the captured text into <var> (or simply throw it away if <var>
	// is "_"), and make sure the "fmt" package's notion of "where to print"
	// is back in sync with the real writer.
	c.b.Emit(bytecode.DropToMarker, tryMarker)
	c.b.Emit(bytecode.EndCapture)

	if isDiscard {
		c.b.Emit(bytecode.Drop, 1)
	} else {
		c.b.Emit(bytecode.Store, varName)
	}

	c.b.Emit(bytecode.SyncOutputWriter)

	// Jump over the catch-handler code below -- it must only run when an
	// error actually occurred.
	b2 := c.b.Mark()
	c.b.Emit(bytecode.Branch, 0)

	// Step 7b: the CATCH path. This is where execution lands if an error
	// occurred anywhere in the block (patched in now that we know the
	// address). This code is NOT user-visible Ego source -- there's no
	// "catch(e)" in @capture's own syntax -- it's generated automatically.
	_ = c.b.SetAddressHere(b1)

	// Stop capturing. The captured text (whatever was printed before the
	// error) is now on top of the value stack.
	c.b.Emit(bytecode.EndCapture)

	// Save it into <var>, so it isn't lost even though the block didn't
	// finish normally -- UNLESS <var> is "_", in which case there is no
	// variable to save it into (see the "Discarding" section of this
	// function's doc comment). Either way, a developer should still SEE the
	// partial output on an error -- discarding on success doesn't mean
	// "never show me this" -- so the text is printed below regardless.
	// Dup makes a second copy of it on the stack before Store consumes the
	// first one, so the copy used for printing doesn't require reading the
	// variable back afterward.
	if !isDiscard {
		c.b.Emit(bytecode.Dup)
		c.b.Emit(bytecode.Store, varName)
	}

	// Print a small, clearly labeled report of what was captured, so a
	// developer watching the console (or looking at "ego test" output) sees
	// it even though nothing in the Ego program explicitly printed it.
	// Print/Newline write straight to the real writer that EndCapture just
	// restored, so this is safe to do right away. The heading is pushed
	// AFTER the captured text, so it ends up on top and prints first; the
	// two Print instructions below each pop exactly one item.
	c.b.Emit(bytecode.Push, "@capture "+varName+":\n")
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Newline)

	// Read the error value NOW, while we are still in the same scope
	// try/catch's machinery stored it in (see handleCatch in catch.go) --
	// reading it later, after PopScope below, could fail to find it.
	c.b.Emit(bytecode.Load, defs.ErrorVariable)

	// Close the block's own scope. This mirrors the extra, explicit
	// PopScope compileTry emits after its catch body: because control
	// jumped directly here instead of falling off the end of the block,
	// the block's own scope was never closed the normal way, so it has to
	// be closed by hand exactly once. This does NOT disturb the value
	// stack, so the error value pushed just above survives it untouched.
	c.b.Emit(bytecode.PopScope)

	// NOW that the surviving, outer scope is current again, tell the "fmt"
	// package which writer to use. Doing this any earlier would record the
	// fix in the scope we just abandoned, silently losing it -- see the
	// long comment in bytecode/capture.go for the full explanation of why
	// this step cannot be combined with EndCapture above.
	c.b.Emit(bytecode.SyncOutputWriter)

	// Finally, re-raise the same error so it keeps propagating outward --
	// to a real enclosing try/catch if there is one, or all the way out to
	// end the program if there isn't. This is the same Throw instruction
	// the "throw" statement itself compiles to (see throw.go).
	c.b.Emit(bytecode.Throw)

	// Both paths converge here.
	_ = c.b.SetAddressHere(b2)
	c.b.Emit(bytecode.TryPop)

	return nil
}
