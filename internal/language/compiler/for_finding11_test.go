package compiler

// Tests for PERFORMANCE.md Finding 11: sharing a single loop-body scope for
// the whole loop, even when the body declares simple top-level locals with
// ":=", by compiling those declarations to the non-erroring SymbolOptCreate
// opcode instead of SymbolCreate. See the large comment block above
// loopBodyIdempotentDeclEligible in for.go for the full design rationale.

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// idempotentDeclEligible is a small test helper that positions a fresh
// Compiler's tokenizer at the start of body (which must be a complete
// "{...}" block, exactly what loopBodyIdempotentDeclEligible expects to see
// via Peek(1)/Tokens[pos]) and returns the predicate's verdict.
func idempotentDeclEligible(t *testing.T, body string) bool {
	t.Helper()

	c := &Compiler{}
	c.t = tokenizer.New(body, true)

	// Update, the compiler uses the local optimization level, and since
	// this compiler wasn't created with New(), must set the opt level
	// to the global value to bre able to see the effects.
	c.optimizationLevel = settings.GetInt(defs.OptimizerSetting)

	return c.loopBodyIdempotentDeclEligible()
}

func TestLoopBodyIdempotentDeclEligible(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "single top-level short declaration, used once, is eligible",
			body: `{ doubled := i * 2; sum = sum + doubled }`,
			want: true,
		},
		{
			name: "two distinct top-level short declarations are both eligible",
			body: `{ zr2 := zr * zr; zi2 := zi * zi; if zr2+zi2 > 4.0 { return i } }`,
			want: true,
		},
		{
			name: "declaration nested inside an if is not disqualifying (and not needed)",
			body: `{ if i > 0 { doubled := i * 2; sum = sum + doubled } }`,
			want: true,
		},
		{
			name: "same name declared twice at top level is not eligible",
			body: `{ y := i; y := i + 1; sum = sum + y }`,
			want: false,
		},
		{
			name: "multi-target := declares two distinct names, still eligible",
			body: `{ a, b := f(); sum = a + b }`,
			want: true,
		},
		{
			name: "discarded name never conflicts with itself",
			body: `{ _ := f(); _ := g() }`,
			want: true,
		},
		{
			name: "var declaration is not eligible (falls back to fresh scope)",
			body: `{ var doubled int; doubled = i * 2 }`,
			want: false,
		},
		{
			name: "function literal is not eligible",
			body: `{ x := i; funcs = append(funcs, func() int { return x }) }`,
			want: false,
		},
		{
			name: "go statement is not eligible",
			body: `{ x := i; go worker(x) }`,
			want: false,
		},
		{
			name: "defer statement is not eligible",
			body: `{ x := i; defer cleanup(x) }`,
			want: false,
		},
		{
			name: "nested for loop's own init clause is not eligible",
			body: `{ x := i; for j := 0; j < i; j++ { sum = sum + j } }`,
			want: false,
		},
		{
			name: "nested switch's own init clause is not eligible",
			body: `{ x := i; switch n := f(); n { default: sum = n } }`,
			want: false,
		},
		{
			name: "no declarations at all is vacuously eligible",
			// In real use this case never reaches
			// loopBodyIdempotentDeclEligible at all: Finding 4's own
			// loopBodyNeedsFreshScopePerIteration already returns false for
			// it, and callers only consult this function when that one
			// returned true. Included here only to document the function's
			// own contract in isolation: "every top-level declared name
			// appears exactly once" is vacuously true with zero declarations.
			body: `{ sum = sum + i }`,
			want: true,
		},
		{
			name: "malformed input with no opening brace is not eligible",
			body: `sum = sum + i`,
			want: false,
		},
	}

	// Optimizer must be enabled for the loop opts to function.
	settings.SetDefault(defs.OptimizerSetting, "2")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := idempotentDeclEligible(t, tt.body)
			if got != tt.want {
				t.Errorf("loopBodyIdempotentDeclEligible(%q) = %v, want %v", tt.body, got, tt.want)
			}
		})
	}
}

// TestFinding11LocalDeclarationsAcrossIterations guards the correctness risk
// identified while implementing Finding 11: a loop body that declares one or
// more simple locals with ":=" every iteration must keep computing the same
// result whether or not the runtime scope backing those declarations is
// fresh every iteration or shared and reused, AND a genuine same-scope
// double declaration (a real bug) must still be rejected.
func TestFinding11LocalDeclarationsAcrossIterations(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		want    any
		wantErr bool
	}{
		{
			name: "classic for loop reusing scope with a single top-level declaration",
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
			name: "classic for loop reusing scope with two top-level declarations, mandelbrot-shaped",
			text: `
				func escape(cr, ci float64) int {
					var zr, zi float64
					maxIter := 25
					for i := 0; i < maxIter; i++ {
						zr2 := zr * zr
						zi2 := zi * zi
						if zr2+zi2 > 4.0 {
							return i
						}
						zi = 2.0*zr*zi + ci
						zr = zr2 - zi2 + cr
					}
					return maxIter
				}
				result := escape(0.3, 0.3) + escape(-1.0, 0.0)
			`,
			want: 50,
		},
		{
			name: "range loop reusing scope with a single top-level declaration",
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
			name: "simple for{} loop reusing scope with a single top-level declaration",
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
			name: "genuine same-scope double declaration is still rejected at runtime",
			text: `
				for i := 0; i < 3; i++ {
					y := i
					y := i + 1
					_ = y
				}
				result := 0
			`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := symbols.NewRootSymbolTable(tt.name)
			AddStandard(s)

			err := RunString(tt.name, s, tt.text)

			if tt.wantErr {
				if errors.Nil(err) {
					t.Fatalf("expected a runtime error for a genuine duplicate declaration, got none")
				}

				return
			}

			if !errors.Nil(err) && err.Error() != errors.ErrStop.Error() {
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
