package compiler

import (
	"testing"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// TestCompileBlockDirectiveDoesNotLeakUnusedVarsSetting is a regression test
// for a bug found while adding tests for BUG-25 (see docs/ISSUES.md BUG-54).
//
// The @compile directive lets an Ego program compile a fragment of code at
// runtime and inspect any compile error. When the fragment does not specify
// its own "unused=true/false" flag, @compile is supposed to fall back to
// whatever the *current* "unused variable is an error" setting
// (defs.UnusedVarsSetting) already is, and restore that same setting when it
// is done - leaving global state exactly as it found it.
//
// The bug: compileBlockDirective() read and restored the wrong settings key
// (defs.UnusedVarLoggingSetting - a verbose-logging toggle - instead of
// defs.UnusedVarsSetting, the actual error-enforcement toggle). If the two
// settings ever disagreed, running so much as one @compile statement would
// silently overwrite defs.UnusedVarsSetting with the *logging* setting's
// value for the rest of the process, corrupting every compile after it.
//
// This test sets the two settings to deliberately different values, runs an
// Ego program containing a @compile block, and checks that
// defs.UnusedVarsSetting is unchanged afterward.
func TestCompileBlockDirectiveDoesNotLeakUnusedVarsSetting(t *testing.T) {
	// Save the real settings so this test cannot leak state into any test
	// that runs after it, regardless of pass/fail.
	previousUnusedVars := settings.Get(defs.UnusedVarsSetting)
	previousUnusedVarLogging := settings.Get(defs.UnusedVarLoggingSetting)

	defer func() {
		settings.SetDefault(defs.UnusedVarsSetting, previousUnusedVars)
		settings.SetDefault(defs.UnusedVarLoggingSetting, previousUnusedVarLogging)
	}()

	// Deliberately set these two (unrelated) settings to different values.
	// If compileBlockDirective() is reading/restoring the wrong one, this
	// mismatch is what exposes the bug.
	settings.SetDefault(defs.UnusedVarsSetting, defs.False)
	settings.SetDefault(defs.UnusedVarLoggingSetting, defs.True)

	// A trivial @compile block; its content does not matter for this test -
	// only the fact that a @compile statement ran at all.
	program := `
		@compile {
			func harmless() {
				result := 1
				fmt.Println(result)
			}
		} catch(e) {
		}
	`

	s := symbols.NewRootSymbolTable(t.Name())
	if err := RunString(t.Name(), s, program); !errors.Nil(err) {
		t.Fatalf("unexpected error running program: %v", err)
	}

	if got := settings.GetBool(defs.UnusedVarsSetting); got != false {
		t.Errorf("defs.UnusedVarsSetting leaked: got %v, want false (its value before @compile ran)", got)
	}
}

// TestCompileBlockDirectiveUnusedFalseSuppressesError is the primary BUG-54
// regression test.
//
// "@compile ... unused=false" is documented (docs/LANGUAGE.md) to disable
// unused-variable checking for the compiled block, but in practice the
// compile error was still reported. Root cause: Compiler.Errors() swept
// every still-open scope's usage errors into c.symbolErrors unconditionally,
// without checking c.flags.unusedVars. PopSymbolScope (called for every
// nested scope: if/else arms, loop bodies, etc.) honors the flag correctly,
// but the compiled block's own top-level scope is never popped by
// PopSymbolScope -- it is only ever swept up by Errors() itself, at Close()
// -- so unused=false had no effect on exactly the scope most tests exercise.
//
// Before the fix, this program's catch(e) block would run because the
// compile failed with "variable created but never used: neverUsed", even
// though unused=false was given.
func TestCompileBlockDirectiveUnusedFalseSuppressesError(t *testing.T) {
	program := `
		caught := false

		@compile block unused=false {
			neverUsed := 42
		} catch(e) {
			caught = true
		}
	`

	s := symbols.NewRootSymbolTable(t.Name())
	if err := RunString(t.Name(), s, program); !errors.Nil(err) {
		t.Fatalf("unexpected error running program: %v", err)
	}

	if v, ok := s.Get("caught"); !ok || v != false {
		t.Errorf("caught = %v (found=%v), want false -- unused=false must suppress the compile error", v, ok)
	}
}

// TestCompileBlockDirectiveUnusedTrueStillReportsError is the companion to
// TestCompileBlockDirectiveUnusedFalseSuppressesError: it confirms the BUG-54
// fix (gating Errors()'s sweep of the block's own top-level scope on
// c.flags.unusedVars) did not disable unused-variable detection altogether --
// only honored an explicit unused=false override. With unused=true, the
// compile error must still be caught.
func TestCompileBlockDirectiveUnusedTrueStillReportsError(t *testing.T) {
	program := `
		caught := false

		@compile block unused=true {
			neverUsed := 42
		} catch(e) {
			caught = true
		}
	`

	s := symbols.NewRootSymbolTable(t.Name())
	if err := RunString(t.Name(), s, program); !errors.Nil(err) {
		t.Fatalf("unexpected error running program: %v", err)
	}

	if v, ok := s.Get("caught"); !ok || v != true {
		t.Errorf("caught = %v (found=%v), want true -- unused=true must still report the compile error", v, ok)
	}
}

// TestCompileBlockDirectiveNestedBraceInBlockMode is the core BUG-39
// regression test.
//
// compileBlockDirective collects the tokens between an "@compile block {"
// directive's opening brace and its matching closing brace by counting
// braces: it starts at 1 (for the opening brace, already consumed) and is
// supposed to stop only when the count returns to 0. The bug used the
// condition "braces <= 1", which instead stopped as soon as the count merely
// returned to 1 -- exactly what happens the moment ANY nested brace pair
// inside the block (here, an "if" body) closes. That silently truncated the
// collected token stream partway through the block, leaving the rest of the
// source -- including the trailing "catch(e) { ... }" clause and anything
// after it -- to be parsed as if the @compile statement had already ended,
// which de-synchronized the whole remaining parse.
//
// Before the fix, running this program failed to compile at all (the
// "catch" keyword was rejected as an unexpected token). After the fix it
// must compile and run to completion.
func TestCompileBlockDirectiveNestedBraceInBlockMode(t *testing.T) {
	// x is declared INSIDE the block (as in the original BUG-39 reproducer)
	// rather than mutated from the outer scope: block-mode @compile content
	// is compiled by an isolated sub-compiler that does not share the outer
	// scope's static variable knowledge, so an outer variable assigned only
	// through a nested "if" inside the block trips an unrelated "unused
	// variable" quirk in that sub-compiler's own scope tracking. That is
	// not what this test is checking -- it only cares whether the token
	// stream itself survives a nested brace intact.
	program := `
		@compile block {
			x := 1
			if true {
				x = 2
			}
			fmt.Println(x)
		} catch(e) {
		}
		y := 99
	`

	s := symbols.NewRootSymbolTable(t.Name())
	if err := RunString(t.Name(), s, program); !errors.Nil(err) {
		t.Fatalf("BUG-39: nested brace inside @compile block corrupted parsing: %v", err)
	}

	// y must be defined and equal to 99: a statement lexically AFTER the
	// entire "@compile ... catch(e) { ... }" construct must still be seen
	// and compiled as ordinary code, not corrupted by the truncated token
	// collection.
	if v, ok := s.Get("y"); !ok || v != 99 {
		t.Errorf("y = %v (found=%v), want 99 -- code after @compile...catch must still compile and run", v, ok)
	}
}

// TestCompileBlockDirectiveTwoTopLevelConstructsFullProgramMode is
// the BUG-39 regression test for the full-program (non-"block")
// form of @compile, which shares the same brace-counting defect.
//
// The notes accompanying BUG-39 explain that the full-program form used to
// appear to work for a single top-level construct (e.g. one function
// declaration) only by accident: the same "braces <= 1" bug dropped that
// construct's own closing brace from the collected tokens, but a
// compensating "tokens.Append(tokenizer.BlockEndToken)" happened to patch
// exactly one dropped brace back on. That coincidence breaks down the
// moment there is a SECOND top-level braced construct, because now there
// are two dropped closing braces and only one is ever patched back.
//
// The fix removes the brace-counting bug (and, with it, the now-unnecessary
// compensating patch), so both function declarations below must compile
// correctly and be callable afterward.
func TestCompileBlockDirectiveTwoTopLevelConstructsFullProgramMode(t *testing.T) {
	program := `
		@compile {
			func first() int {
				return 1
			}
			func second() int {
				return 2
			}
		} catch(e) {
		}
		total := first() + second()
	`

	s := symbols.NewRootSymbolTable(t.Name())
	if err := RunString(t.Name(), s, program); !errors.Nil(err) {
		t.Fatalf("BUG-39: two top-level constructs in @compile (full-program mode) corrupted parsing: %v", err)
	}

	if v, ok := s.Get("total"); !ok || v != 3 {
		t.Errorf("total = %v (found=%v), want 3 -- both first() and second() must have compiled and run", v, ok)
	}
}
