package compiler

// Regression tests for two bugs found and fixed together while investigating
// PERFORMANCE.md's goroutine-parallel Mandelbrot workload, both specific to
// running with the bytecode optimizer active (ego.compiler.optimize == 2,
// the "always optimize" level; also reachable at level 1 for bytecode large
// enough to cross the conditional-optimization threshold):
//
//  1. assignmentTarget (lvalue.go) calls bc.Seal() on the small "lvalue"
//     bytecode buffer it returns before the caller ever sees it. When the
//     optimizer is active, this can collapse a simple "for i := 0; ..."
//     loop counter's "SymbolCreate \"i\"; Store \"i\"" into a single
//     CreateAndStore "i" instruction before iterationFor's BUG-30
//     isSimpleIndex check (for.go) ever inspects it. That check only
//     recognized the two un-optimized shapes, so it silently classified an
//     entirely ordinary loop counter as "not simple" and skipped emitting
//     the per-iteration closure-capture prologue/epilogue - breaking
//     per-iteration loop variable semantics for every "for i := 0; ..."
//     loop with a closure, whenever the optimizer ran. Fixed by also
//     recognizing bytecode.CreateAndStore in isSimpleIndex's switch.
//
//  2. Independently, internal/language/bytecode/optimizations.go's
//     "Collapse constant Push and CreateAndStore" rule could re-fire on its
//     own already-folded output and, in doing so, match a Push of a
//     StackMarker frame sentinel as if it were "the value" to fold -
//     corrupting the temp variable's name and silently deleting a marker
//     push a later DropToMarker/TryPop still needed. This surfaced through
//     the "?expr : fallback" optional operator (a Go-level regression test
//     for that fix lives in the bytecode package, alongside the
//     optimizer rule itself); the test below exercises it end-to-end
//     through the compiler as well, since it happens to share the same
//     "-o 2" testing setup as the isSimpleIndex fix above.

import (
	"testing"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// withOptimizerAlwaysOn sets ego.compiler.optimize to "2" (always optimize)
// for the duration of the calling test, restoring the previous value on
// cleanup.
func withOptimizerAlwaysOn(t *testing.T) {
	t.Helper()

	previous := settings.Get(defs.OptimizerSetting)
	settings.Set(defs.OptimizerSetting, "2")

	t.Cleanup(func() {
		settings.Set(defs.OptimizerSetting, previous)
	})
}

// TestForLoopClosureCapture_SurvivesOptimizer is a regression test for bug 1
// above: a classic "for i := 0; i < n; i++" loop whose body creates a
// closure over "i" must still capture a distinct value per iteration when
// the optimizer is active, exactly as it already does at optimizer level 0
// (see for_loopvar_test.go / tests/flow/for_loopvar.ego for the original,
// optimizer-independent coverage of this behavior).
func TestForLoopClosureCapture_SurvivesOptimizer(t *testing.T) {
	withOptimizerAlwaysOn(t)

	text := `
		var funcs []any
		for i := 0; i < 3; i++ {
			funcs = append(funcs, func() int { return i })
		}
		a := funcs[0].(func() int)()
		b := funcs[1].(func() int)()
		c := funcs[2].(func() int)()
	`

	s := symbols.NewRootSymbolTable(t.Name())
	AddStandard(s)

	if err := RunString(t.Name(), s, text); !errors.Nil(err) && err.Error() != errors.ErrStop.Error() {
		t.Fatalf("unexpected compile/run error: %v", err)
	}

	for name, want := range map[string]int{"a": 0, "b": 1, "c": 2} {
		got, found := s.Get(name)
		if !found {
			t.Fatalf("program did not set %q", name)
		}

		if got != want {
			t.Errorf("%s = %v, want %v (closures did not capture distinct per-iteration values under the optimizer)", name, got, want)
		}
	}
}

// TestOptionalOperator_SurvivesOptimizer is a regression test for bug 2
// above, exercised end-to-end through the compiler: two uses of the "?expr :
// fallback" optional operator in the same scope, each with a constant-foldable
// success expression, must both still work when the optimizer is active. See
// internal/language/bytecode/optimizer_test.go's
// Test_Optimize_CollapsePushAndCreateAndStore_DoesNotEatStackMarker for a
// lower-level, bytecode-only reproduction of the exact mechanism.
func TestOptionalOperator_SurvivesOptimizer(t *testing.T) {
	withOptimizerAlwaysOn(t)

	previousExtensions := settings.Get(defs.ExtensionsEnabledSetting)
	settings.Set(defs.ExtensionsEnabledSetting, "true")

	t.Cleanup(func() {
		settings.Set(defs.ExtensionsEnabledSetting, previousExtensions)
	})

	text := `
		zero := 0
		a := ?(100/zero) : 99
		b := ?(100/5) : 99
	`

	s := symbols.NewRootSymbolTable(t.Name())
	AddStandard(s)

	if err := RunString(t.Name(), s, text); !errors.Nil(err) && err.Error() != errors.ErrStop.Error() {
		t.Fatalf("unexpected compile/run error: %v", err)
	}

	for name, want := range map[string]int{"a": 99, "b": 20} {
		got, found := s.Get(name)
		if !found {
			t.Fatalf("program did not set %q", name)
		}

		if got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
}
