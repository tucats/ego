package compiler

// Tests for the unused-variable bookkeeping in symbols.go, specifically
// validateSymbol / resolveExternalSymbol / markSymbolAsUsed.
//
// BACKGROUND: while adding Ego-language tests for BUG-46 (docs/ISSUES.md), a
// test needed to branch on the ambient type-checking mode by reading the
// invisible "__type_checking" runtime global (set by staticTypingByteCode,
// see internal/language/bytecode/types.go). A single, one-time read of that
// global from a test's @test block failed to compile with "variable created
// but never used: __type_checking" -- even though the value was plainly
// read and compared. This was a genuine compiler bug, not a test mistake.
//
// ROOT CAUSE: resolveExternalSymbol's "not found anywhere, but mustExist is
// false" branch is reached by two, semantically different call paths that
// both end up with mustExist == false:
//
//  1. ReferenceOrDefineSymbol (only caller: the simple-lvalue path in
//     lvalue.go, e.g. compiling "neverUsed := 42") starts mustExist as
//     false deliberately, because the name may not exist yet -- this call
//     IS the declaration, and correctly needs to register a fresh
//     "declared but not yet used" entry so a later scope-close still
//     catches a truly unused local.
//  2. validateSymbol forces mustExist to false for names the compiler can
//     never see declared at all: generated ("$"-prefixed) names, invisible
//     ("__"-prefixed) runtime globals like __type_checking, trial
//     compilations, and server mode. This call IS a read of something
//     already known to exist at runtime even though no compile-time scope
//     ever declared it.
//
// Both cases used to funnel into the same unconditional c.DefineSymbol(name)
// call, which always registers a fresh "unused until referenced again"
// entry. That is correct for case 1, but wrong for case 2: a case-2 name
// referenced exactly once in the whole compilation unit has no second
// reference to clear the sentinel, so it was always reported unused.
//
// THE FIX: validateSymbol now captures allowImplicitDefine (whether the
// caller's ORIGINAL mustExist argument was already false, i.e. case 1) before
// applying the case-2 exemptions, and threads it through to
// resolveExternalSymbol, which now calls c.DefineSymbol (fresh declaration)
// only when allowImplicitDefine is true, and c.markSymbolAsUsed (mark used
// immediately, no sentinel) otherwise.
//
// These tests use the same @compile/RunString/catch(e) pattern as the
// existing BUG-54 tests in directives_test.go: compile a fragment with
// "unused=true" forced on, and check whether the compile error was caught.
// They use "_ = name" rather than fmt.Println(name) to mark a variable used,
// since the compiler package's own test harness (RunString/AddStandard) does
// not auto-import fmt the way `ego run`/`ego test` do.

import (
	"testing"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// Test_InvisibleGlobalSingleReadNotFlaggedUnused is the primary regression
// test for the bug described above: a lone read of an invisible "__"-prefixed
// runtime global must not be reported as an unused variable.
func Test_InvisibleGlobalSingleReadNotFlaggedUnused(t *testing.T) {
	program := `
		caught := false

		@compile block unused=true {
			_ = __type_checking
		} catch(e) {
			caught = true
		}
	`

	s := symbols.NewRootSymbolTable(t.Name())
	s.SetAlways(defs.TypeCheckingVariable, defs.NoTypeEnforcement)

	if err := RunString(t.Name(), s, program); !errors.Nil(err) {
		t.Fatalf("unexpected error running program: %v", err)
	}

	if v, ok := s.Get("caught"); !ok || v != false {
		t.Errorf("caught = %v (found=%v), want false -- a single read of __type_checking must not be flagged unused", v, ok)
	}
}

// Test_InvisibleGlobalReadTwiceNotFlaggedUnused guards against a narrower
// fix that only worked by accident for exactly one reference: reading the
// same invisible global twice in the same scope must also compile cleanly.
func Test_InvisibleGlobalReadTwiceNotFlaggedUnused(t *testing.T) {
	program := `
		caught := false

		@compile block unused=true {
			_ = __type_checking + __type_checking
		} catch(e) {
			caught = true
		}
	`

	s := symbols.NewRootSymbolTable(t.Name())
	s.SetAlways(defs.TypeCheckingVariable, defs.NoTypeEnforcement)

	if err := RunString(t.Name(), s, program); !errors.Nil(err) {
		t.Fatalf("unexpected error running program: %v", err)
	}

	if v, ok := s.Get("caught"); !ok || v != false {
		t.Errorf("caught = %v (found=%v), want false -- two reads of __type_checking must not be flagged unused", v, ok)
	}
}

// Test_SimpleAssignmentTargetStillFlaggedUnused guards against the
// regression this fix could easily reintroduce in the other direction: a
// genuinely unused local declared via a simple ":=" (which is compiled via
// ReferenceOrDefineSymbol, the other path into the same resolveExternalSymbol
// branch fixed above) must still be reported unused. This is the same
// assertion as TestCompileBlockDirectiveUnusedTrueStillReportsError in
// directives_test.go, repeated here because it is the exact scenario that
// regressed while developing the __type_checking fix above: an earlier,
// overly broad version of the fix made every ":=" target silently skip
// unused-variable detection.
func Test_SimpleAssignmentTargetStillFlaggedUnused(t *testing.T) {
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
		t.Errorf("caught = %v (found=%v), want true -- a genuinely unused local must still be reported", v, ok)
	}
}

// Test_SimpleAssignmentTargetLaterReadNotFlagged verifies the same
// ReferenceOrDefineSymbol path used by Test_SimpleAssignmentTargetStillFlaggedUnused
// correctly clears the "unused" sentinel when the declared variable IS
// subsequently read, so the fix does not overcorrect into flagging normal,
// used local variables.
func Test_SimpleAssignmentTargetLaterReadNotFlagged(t *testing.T) {
	program := `
		caught := false

		@compile block unused=true {
			used := 42
			_ = used
		} catch(e) {
			caught = true
		}
	`

	s := symbols.NewRootSymbolTable(t.Name())
	if err := RunString(t.Name(), s, program); !errors.Nil(err) {
		t.Fatalf("unexpected error running program: %v", err)
	}

	if v, ok := s.Get("caught"); !ok || v != false {
		t.Errorf("caught = %v (found=%v), want false -- a subsequently-read local must not be flagged unused", v, ok)
	}
}
