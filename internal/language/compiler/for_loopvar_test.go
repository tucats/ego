package compiler

// Tests for the BUG-30 fix: "for" and "for range" loops must give each
// iteration its own fresh copy of the loop's index/value variable(s),
// matching the Go 1.22+ language change ("per-iteration loop variables").
//
// Before this fix, a loop's index/value variable(s) lived in a single scope
// that was pushed once before the loop started and popped once after it
// ended - so every iteration shared the exact same variable. A closure
// created inside the loop body therefore always observed whatever value the
// variable held when the loop finished, regardless of which iteration
// created the closure. See docs/ISSUES.md BUG-30 for the full write-up,
// including why this needed both a "prologue" (fresh per-iteration copy,
// always runs) and an "epilogue" (copies a body-side mutation of the loop
// variable back out, best-effort - skipped by break/continue) rather than
// just a fresh copy alone.
//
// A note on how these tests are written: every test that uses "break" or
// "continue" wraps the loop in a named function and checks its return value,
// rather than following the loop with more bare top-level statements. That
// sidesteps an unrelated, pre-existing bug (present before this fix too -
// confirmed by testing against the unmodified compiler) where a "break" or
// "continue" nested inside an "if" block, at the bare top level of a
// RunString fragment (no enclosing function), leaves the compiler's runtime
// scope one level deeper than it should be, so a variable declared by a
// *later* top-level statement ends up trapped in that leftover scope. This
// only reproduces in bare top-level fragments - a real function body, like
// the ones these tests use, does not exhibit it - and is not something this
// fix introduces or is responsible for correcting.

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

func TestBUG30PerIterationLoopVariables(t *testing.T) {
	tests := []struct {
		name string
		text string
		want any
	}{
		{
			// The exact reproducer from docs/ISSUES.md BUG-30.
			name: "classic for loop: closures capture distinct per-iteration values",
			text: `
				var funcs []any
				for i := 0; i < 3; i++ {
					funcs = append(funcs, func() int { return i })
				}
				result := funcs[0]() == 0 && funcs[1]() == 1 && funcs[2]() == 2
			`,
			want: true,
		},
		{
			name: "range loop: closures over the index capture distinct values",
			text: `
				items := []string{"a", "b", "c"}
				var funcs []any
				for i := range items {
					funcs = append(funcs, func() int { return i })
				}
				result := funcs[0]() == 0 && funcs[1]() == 1 && funcs[2]() == 2
			`,
			want: true,
		},
		{
			name: "range loop: closures over the value capture distinct values",
			text: `
				items := []string{"a", "b", "c"}
				var funcs []any
				for _, v := range items {
					funcs = append(funcs, func() string { return v })
				}
				result := funcs[0]() == "a" && funcs[1]() == "b" && funcs[2]() == "c"
			`,
			want: true,
		},
		{
			name: "range loop: closures capture distinct index AND value pairs",
			text: `
				items := []string{"a", "b", "c"}
				var funcs []any
				for i, v := range items {
					funcs = append(funcs, func() string { return string(i) + v })
				}
				result := funcs[0]() == string(0) + "a" && funcs[1]() == string(1) + "b" && funcs[2]() == string(2) + "c"
			`,
			want: true,
		},
		{
			// This is the exact scenario in tests/functions/scope_advanced.ego,
			// "functions: stored closure survives after loop", updated for
			// Go 1.22+ semantics: the closure created on the LAST iteration
			// (i == 2; the loop then increments to 3 and the condition fails)
			// must return that iteration's value (2), not whatever the loop
			// counter held after the loop finished (3).
			name: "closure stored during loop and called after loop ends sees its own iteration's value",
			text: `
				captured := func() int { return -1 }
				for i := 0; i < 3; i = i + 1 {
					captured = func() int { return i }
				}
				result := captured() == 2
			`,
			want: true,
		},
		{
			// Go 1.22 explicitly preserves this pattern: reassigning the loop
			// counter inside the body still affects later iterations, as long
			// as the iteration finishes normally (see loopVariableEpilogue).
			name: "mutating the loop counter inside the body still affects later iterations",
			text: `
				count := 0
				for i := 0; i < 10; i++ {
					count++
					if i == 2 {
						i = 100
					}
				}
				result := count == 3
			`,
			want: true,
		},
		{
			// A closure created just before a "continue" must still capture
			// that iteration's own value - the fix's copy-in step always runs
			// before any user code, including a continue, in the same
			// iteration. Wrapped in a function (see the file-level comment)
			// because of the unrelated, pre-existing top-level fragment bug.
			name: "closure captures correctly even when its iteration also continues",
			text: `
				func run() bool {
					var funcs []any
					for i := 0; i < 5; i++ {
						if i == 2 {
							continue
						}
						funcs = append(funcs, func() int { return i })
					}
					return len(funcs) == 4 && funcs[0]() == 0 && funcs[1]() == 1 && funcs[2]() == 3 && funcs[3]() == 4
				}
				result := run()
			`,
			want: true,
		},
		{
			name: "nested loops: each loop's variable is independently fresh",
			text: `
				var funcs []any
				for i := 0; i < 2; i++ {
					for j := 0; j < 2; j++ {
						funcs = append(funcs, func() int { return i*10 + j })
					}
				}
				result := funcs[0]() == 0 && funcs[1]() == 1 && funcs[2]() == 10 && funcs[3]() == 11
			`,
			want: true,
		},
		{
			name: "loop variable unused in body does not trigger a false unused-variable error",
			text: `
				ticks := 0
				for i := 0; i < 3; i++ {
					ticks++
				}
				result := ticks == 3
			`,
			want: true,
		},
		{
			name: "empty for-loop bodies compile and run without error",
			text: `
				n := 0
				for n < 3 {
					n++
				}
				items := []int{1, 2, 3}
				for i := range items {
				}
				result := n == 3
			`,
			want: true,
		},
		{
			name: "ordinary loop with no closures still computes the correct sum",
			text: `
				sum := 0
				for i := 0; i < 100; i++ {
					sum += i
				}
				result := sum == 4950
			`,
			want: true,
		},
		{
			// break/continue tests are run inside a named function (see the
			// file-level comment) to avoid an unrelated, pre-existing bug
			// with break/continue at the bare top level of a fragment.
			name: "break still exits the loop correctly",
			text: `
				func run() bool {
					total := 0
					for i := 0; i < 5; i++ {
						if i == 3 {
							break
						}
						total = total + i
					}
					return total == 3
				}
				result := run()
			`,
			want: true,
		},
		{
			name: "continue still skips the rest of the body correctly",
			text: `
				func run() bool {
					total := 0
					for i := 0; i < 5; i++ {
						if i == 2 {
							continue
						}
						total = total + i
					}
					return total == 8
				}
				result := run()
			`,
			want: true,
		},
		{
			name: "labeled continue across nested loops still works",
			text: `
				func run() bool {
					outerCount := 0
					outer:
					for i := 0; i < 3; i++ {
						for j := 0; j < 3; j++ {
							if j == 1 {
								continue outer
							}
							outerCount++
						}
					}
					return outerCount == 3
				}
				result := run()
			`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := symbols.NewRootSymbolTable(tt.name)
			AddStandard(s)

			if err := RunString(tt.name, s, tt.text); !errors.Nil(err) && err.Error() != errors.ErrStop.Error() {
				t.Fatalf("unexpected compile/run error: %v", err)
			}

			result, found := s.Get("result")
			if !found {
				t.Fatal("program did not set 'result'")
			}

			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("got %v (%T), want %v (%T)", result, result, tt.want, tt.want)
			}
		})
	}
}

// TestBUG30QualifiedLvalueCounterUnaffected verifies that a for-loop whose
// counter is a qualified lvalue (an array element, not a plain name) is left
// exactly as before: this fix only applies when the loop counter is a simple
// named variable, since a qualified lvalue has no single variable name to
// give a fresh per-iteration copy to. This must still compile and run
// without error.
func TestBUG30QualifiedLvalueCounterUnaffected(t *testing.T) {
	s := symbols.NewRootSymbolTable(t.Name())
	AddStandard(s)

	text := `
		a := []int{0}
		count := 0
		for a[0] = 0; a[0] < 3; a[0]++ {
			count++
		}
		result := count == 3 && a[0] == 3
	`

	if err := RunString(t.Name(), s, text); !errors.Nil(err) && err.Error() != errors.ErrStop.Error() {
		t.Fatalf("unexpected compile/run error: %v", err)
	}

	result, found := s.Get("result")
	if !found {
		t.Fatal("program did not set 'result'")
	}

	if result != true {
		t.Errorf("got %v, want true", result)
	}
}
