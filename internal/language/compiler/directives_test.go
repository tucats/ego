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
