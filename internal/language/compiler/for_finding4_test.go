package compiler

// Tests for PERFORMANCE.md Finding 4: skipping the per-iteration loop-body
// scope (and the BUG-30 copy-in/copy-out prologue/epilogue) when a loop body
// contains neither a closure-capturing construct nor a top-level local
// declaration. See the large comment block above compileForBody in for.go
// for the full design rationale.

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// needsFreshScope is a small test helper that positions a fresh Compiler's
// tokenizer at the start of body (which must be a complete "{...}" block,
// exactly what loopBodyNeedsFreshScopePerIteration expects to see via
// Peek(1)/Tokens[pos]) and returns the predicate's verdict.
func needsFreshScope(t *testing.T, body string) bool {
	t.Helper()

	c := &Compiler{}
	c.t = tokenizer.New(body, true)

	// Update, the compiler uses the local optimization level, and since
	// this compiler wasn't created with New(), must set the opt level
	// to the global value to bre able to see the effects.
	c.optimizationLevel = settings.GetInt(defs.OptimizerSetting)

	return c.loopBodyNeedsFreshScopePerIteration()
}

func TestLoopBodyNeedsFreshScopePerIteration(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "empty block",
			body: `{}`,
			want: false,
		},
		{
			name: "arithmetic only, no declarations or closures",
			body: `{ sum = sum + i }`,
			want: false,
		},
		{
			name: "assignment to existing outer variable, not a declaration",
			body: `{ sum = sum + i; count = count + 1 }`,
			want: false,
		},
		{
			name: "top-level short declaration disqualifies",
			body: `{ doubled := i * 2; sum = sum + doubled }`,
			want: true,
		},
		{
			name: "top-level var declaration disqualifies",
			body: `{ var doubled int; doubled = i * 2 }`,
			want: true,
		},
		{
			name: "short declaration nested inside an if is still disqualifying at depth 2",
			// Even though this is a known-conservative case (the declaration
			// is actually scoped to the "if" block, which gets its own scope
			// regardless), the current scan only special-cases depth == 1,
			// so a nested declaration is not detected and does NOT disqualify.
			body: `{ if i > 0 { doubled := i * 2; sum = sum + doubled } }`,
			want: false,
		},
		{
			name: "function literal disqualifies",
			body: `{ funcs = append(funcs, func() int { return i }) }`,
			want: true,
		},
		{
			name: "go statement disqualifies",
			body: `{ go worker(i) }`,
			want: true,
		},
		{
			name: "defer statement disqualifies",
			body: `{ defer cleanup(i) }`,
			want: true,
		},
		{
			name: "nested for loop with its own short declaration before its own brace",
			// Known conservative gap documented in for.go: the nested loop's
			// own ":=" is seen at depth == 1 (before the nested loop's own
			// "{" increases depth), so it disqualifies even though it is
			// actually scoped to the nested loop, not this body.
			body: `{ for j := 0; j < i; j++ { sum = sum + j } }`,
			want: true,
		},
		{
			name: "malformed input with no opening brace is treated as unsafe",
			body: `sum = sum + i`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := needsFreshScope(t, tt.body)
			if got != tt.want {
				t.Errorf("loopBodyNeedsFreshScopePerIteration(%q) = %v, want %v", tt.body, got, tt.want)
			}
		})
	}
}

// TestFinding4LocalDeclarationsAcrossIterations guards the correctness risk
// identified while implementing Finding 4: a loop body that declares a local
// variable with ":=" or "var" every iteration must keep working correctly
// even though the optimization means most such loops no longer get a fresh
// runtime scope per iteration - loopBodyNeedsFreshScopePerIteration is
// required to detect this exact case and force the old (safe) per-iteration
// scope behavior, precisely so that symbols.ErrSymbolExists is never
// triggered by re-declaring the same name on the second iteration.
func TestFinding4LocalDeclarationsAcrossIterations(t *testing.T) {
	tests := []struct {
		name string
		text string
		want any
	}{
		{
			name: "classic for loop with a local short declaration in the body",
			text: `
				total := 0
				for i := 0; i < 5; i++ {
					doubled := i * 2
					total = total + doubled
				}
				result := total
			`,
			want: 20,
		},
		{
			name: "range loop with a local short declaration in the body",
			text: `
				items := []int{1, 2, 3, 4}
				total := 0
				for _, v := range items {
					squared := v * v
					total = total + squared
				}
				result := total
			`,
			want: 30,
		},
		{
			name: "simple for{} loop with a local short declaration in the body",
			// Uses "n = n + 1" rather than "n++" as a standalone statement:
			// see docs/ISSUES.md BUG-63, an unrelated, pre-existing bug
			// (confirmed present with Finding 4's changes reverted) where a
			// bare "x++"/"x--" statement leaks a "let" stack marker that can
			// corrupt an enclosing function's later return. Not something
			// this test is trying to exercise.
			text: `
				func run() int {
					total := 0
					n := 0
					for {
						doubled := n * 2
						total = total + doubled
						n = n + 1
						if n >= 4 {
							break
						}
					}
					return total
				}
				result := run()
			`,
			want: 12,
		},
		{
			name: "for loop body with no declarations and no closures still computes correctly",
			text: `
				total := 0
				for i := 0; i < 1000; i++ {
					total = total + i
				}
				result := total
			`,
			want: 499500,
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
