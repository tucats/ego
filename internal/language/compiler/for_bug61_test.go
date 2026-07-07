package compiler

// Tests for docs/ISSUES.md BUG-61: compileBreak/compileContinue (for.go)
// used to emit a bare, unconditional Branch with no scope cleanup at all,
// silently leaving open whatever scopes had been pushed by blocks, switch
// statements, etc. nested between the break/continue and its target loop.
// The runtime's "current scope" pointer ended up one or more levels too
// deep after the branch, corrupting later variable lookups/declarations
// (Reproducer 1) or, for a named-init switch, leaking that switch's own
// init-variable scope on every "continue" that fired inside it
// (Reproducer 2). See the large design comment on c.scopeDepth in
// compiler.go, and on emitScopeUnwindTo in for.go, for the fix.

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// TestBUG61Reproducer1ScopeUnwindOutsideFunction is BUG-61's own original
// reproducer: a multi-statement program with no enclosing "func main()"
// (RunString's fragment mode), containing a "for" loop with a "break"
// nested inside an "if" block. Before the fix, "result" (declared by the
// statement immediately after the loop) was never visible to the caller's
// symbol table, because the break left the runtime's current scope one
// level too deep.
//
// Every "if" body here deliberately declares a throwaway local
// ("marker := true; _ = marker"). Without a declaration of its own, an "if"
// body's scope is skipped entirely by PERFORMANCE.md Finding 8 (added in a
// later session than BUG-61 itself) - with no scope pushed for the break to
// skip over in the first place, a bare, non-declaring "if { break }" no
// longer demonstrates this specific bug on today's compiler, even without
// the BUG-61 fix. Forcing a real (non-elided) scope keeps this test
// meaningful regardless of Finding 8; the underlying defect this guards
// against is compileBreak/compileContinue failing to unwind any real scope
// that happens to sit between them and their target loop - an "if" body is
// simply the simplest such scope, not the only one (see the switch/try
// tests elsewhere in this file for others).
func TestBUG61Reproducer1ScopeUnwindOutsideFunction(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{
			name: "break nested inside a declaring if",
			text: `
				total := 0
				for i := 0; i < 5; i++ {
					if i == 3 {
						marker := true
						_ = marker
						break
					}
					total = total + i
				}
				result := total == 3
			`,
		},
		{
			name: "continue nested inside a declaring if",
			text: `
				total := 0
				for i := 0; i < 5; i++ {
					if i == 3 {
						marker := true
						_ = marker
						continue
					}
					total = total + i
				}
				result := total == 7
			`,
		},
		{
			name: "break nested two levels deep, inner if declaring",
			text: `
				total := 0
				for i := 0; i < 5; i++ {
					if true {
						if i == 3 {
							marker := true
							_ = marker
							break
						}
					}
					total = total + i
				}
				result := total == 3
			`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := symbols.NewRootSymbolTable(t.Name())
			AddStandard(s)

			err := RunString(t.Name(), s, tt.text)
			if !errors.Nil(err) {
				t.Fatalf("unexpected compile/run error: %v", err)
			}

			v, found := s.Get("result")
			if !found {
				t.Fatal("'result' was not visible in the caller's symbol table after the loop - scope leak")
			}

			if v != true {
				t.Errorf("got result=%v, want true", v)
			}
		})
	}
}

// TestBUG61Reproducer2NamedInitSwitchContinueDoesNotLeak is BUG-61's second
// reproducer: a named-init switch ("switch v := expr; v { ... }") inside a
// loop, where a "continue" inside a case body used to leak the switch's own
// scope (holding "v") every time it fired, so a later, unrelated "v" in the
// same enclosing scope failed with "symbol already exists". This reproduces
// through the ordinary ego run/RunString path, not just a Go-level harness.
func TestBUG61Reproducer2NamedInitSwitchContinueDoesNotLeak(t *testing.T) {
	s := symbols.NewRootSymbolTable(t.Name())
	AddStandard(s)

	text := `
		func run() int {
			count := 0
			for i := 0; i < 6; i++ {
				switch v := i; v {
				case 2, 4:
					continue
				}
				count++
			}

			v := 1000
			return count + v
		}

		result := run()
	`

	if err := RunString(t.Name(), s, text); !errors.Nil(err) && err.Error() != errors.ErrStop.Error() {
		t.Fatalf("unexpected compile/run error: %v", err)
	}

	result, found := s.Get("result")
	if !found {
		t.Fatal("program did not set 'result'")
	}

	// i = 0..5; i = 2 and 4 are skipped by "continue", leaving 4 increments
	// (i = 0, 1, 3, 5). count=4, v=1000 declared after the loop with no
	// collision -> 1004.
	if !reflect.DeepEqual(result, 1004) {
		t.Errorf("got %v (%T), want 1004", result, result)
	}
}

// TestBUG61AnonymousSwitchContinueDoesNotLeakSyntheticSymbol guards the
// second half of BUG-61: an ANONYMOUS switch ("switch expr { ... }", no
// named init) used to create its synthetic test-value symbol directly in
// the enclosing scope and clean it up with an explicit SymbolDelete at the
// switch's normal exit - a "continue" inside a case body could skip that
// SymbolDelete. The fix makes every switch form push a real scope
// uniformly, so this is now covered by the same scope-unwind mechanism as
// the named-init case; this test exercises the anonymous form specifically,
// many times in a row, to catch any accumulating leak.
//
// Note: because the leaked name in the anonymous form is a compiler-
// generated placeholder (data.GenerateName(), unique per switch statement in
// the source, not a user-visible name like Reproducer 2's "v"), this
// specific test is not guaranteed to fail on the pre-fix compiler the way
// Reproducer 2 is - there is nothing else in this program that could
// collide with that placeholder. It is kept as a targeted regression test
// for the anonymous form's own code path (and passed as an early sanity
// check while developing the fix), on top of Reproducer 2's stronger,
// collision-based guarantee for the named-init form.
func TestBUG61AnonymousSwitchContinueDoesNotLeakSyntheticSymbol(t *testing.T) {
	s := symbols.NewRootSymbolTable(t.Name())
	AddStandard(s)

	text := `
		func attempt(n int) int {
			count := 0
			for i := 0; i < 9; i++ {
				switch i {
				case 2, 5, 8:
					continue
				}
				count++
			}
			return count + n
		}

		total := 0
		for k := 0; k < 500; k++ {
			total = total + attempt(k)
		}

		result := total
	`

	if err := RunString(t.Name(), s, text); !errors.Nil(err) && err.Error() != errors.ErrStop.Error() {
		t.Fatalf("unexpected compile/run error: %v", err)
	}

	result, found := s.Get("result")
	if !found {
		t.Fatal("program did not set 'result'")
	}

	// attempt(n): i=0..8 (9 values), 3 skipped by continue (2,5,8) -> count=6.
	// Sum over k=0..499 of (6+k) = 500*6 + sum(0..499) = 3000 + 124750 = 127750.
	if !reflect.DeepEqual(result, 127750) {
		t.Errorf("got %v (%T), want 127750", result, result)
	}
}

// TestBUG61SimpleForClosesOwnForScope guards a related defect found while
// implementing the general fix: simpleFor (a bare "for {}" loop) never
// popped compileFor's own outer "ForScope" at all - unlike the other three
// loop forms, which all do. This permanently leaked one scope level every
// time a bare "for {}" ran.
//
// The loops here are deliberately run directly at the top level, NOT each
// wrapped in its own function call: a leak that stays entirely within a
// single function call is invisible to the caller, because Return
// (callFramePop) resets the runtime's current-scope pointer directly from
// the saved call frame, unconditionally overwriting any internal drift. Only
// a leak that reaches all the way back out to the top-level/fragment scope -
// exactly what happens here, and exactly the shape that would break
// compileBreak/compileContinue's c.scopeDepth bookkeeping for any code
// textually following such a loop - is guaranteed to be observable.
func TestBUG61SimpleForClosesOwnForScope(t *testing.T) {
	s := symbols.NewRootSymbolTable(t.Name())
	AddStandard(s)

	text := `
		a := 0
		for {
			a = a + 1
			if a >= 3 {
				break
			}
		}

		b := 0
		for {
			b = b + 1
			if b >= 5 {
				break
			}
		}

		c := 0
		for {
			c = c + 1
			if c >= 2 {
				break
			}
		}

		result := a + b + c
	`

	if err := RunString(t.Name(), s, text); !errors.Nil(err) && err.Error() != errors.ErrStop.Error() {
		t.Fatalf("unexpected compile/run error: %v", err)
	}

	result, found := s.Get("result")
	if !found {
		t.Fatal("program did not set 'result' - scope leak from a preceding bare \"for {}\" loop")
	}

	if !reflect.DeepEqual(result, 10) {
		t.Errorf("got %v (%T), want 10", result, result)
	}
}

// TestBUG61LabeledBreakContinueAcrossMixedNesting is a correctness/stress
// test for the general scope-unwind mechanism: a labeled "continue"/"break"
// fired from deep inside a mix of "if", "switch", "try"/"catch", and a
// nested "for" loop, all wrapped in an outer labeled loop. This exercises
// unwinding through several different kinds of scope-pushing constructs at
// once, mirroring real-world code far more than any single-construct test
// can, and confirms the fix produces exactly the right values under that
// complexity (verified by hand-tracing every iteration - see the comments
// below).
//
// Unlike the Reproducer 1/2 tests above, this one is not guaranteed to fail
// without the fix: every value this program touches after the point a
// "continue outer" fires (append to an existing slice variable, loop
// control variables read/written by their own fixed-position bytecode) is
// naturally tolerant of the runtime's current-scope pointer being left too
// deep - reads and writes to an EXISTING name still succeed by walking up
// the (deeper than expected) parent chain, so nothing here happens to
// re-declare a name at a level the drift would corrupt. It is included
// anyway as a realistic stress test, on top of the two reproducers above,
// which do reliably discriminate old from new behavior.
func TestBUG61LabeledBreakContinueAcrossMixedNesting(t *testing.T) {
	s := symbols.NewRootSymbolTable(t.Name())
	AddStandard(s)

	text := `
		func risky(v int) int {
			if v % 7 == 0 {
				return 1 / 0
			}
			return v
		}

		func run() []int {
			var found []int

		outer:
			for i := 0; i < 6; i++ {
				for j := 0; j < 6; j++ {
					if true {
						switch j {
						case 3, 4:
							try {
								_ = risky(i*10 + j)
							} catch {
								continue outer
							}
							if i == 4 {
								break outer
							}
						}
					}
					found = append(found, i*10+j)
				}
			}

			return found
		}

		found := run()
		result := len(found)
		last := found[len(found)-1]
	`

	if err := RunString(t.Name(), s, text); !errors.Nil(err) && err.Error() != errors.ErrStop.Error() {
		t.Fatalf("unexpected compile/run error: %v", err)
	}

	result, found := s.Get("result")
	if !found {
		t.Fatal("program did not set 'result'")
	}

	last, foundLast := s.Get("last")
	if !foundLast {
		t.Fatal("program did not set 'last'")
	}

	// i=0: j=0..5, all appended (6 values: 0,1,2,3,4,5).
	// i=1: j=4 -> risky(14) errors (14%7==0) -> continue outer; j=0..3
	//      appended first (10,11,12,13), so 4 values for i=1.
	// i=2: j=0..5, all appended (6 values: 20..25).
	// i=3: j=0..5, all appended (6 values: 30..35).
	// i=4: j=0..2 appended (40,41,42), then j=3 -> risky(43) does not error
	//      (43%7==1) -> falls through to "if i==4 { break outer }" -> exits
	//      both loops entirely. 3 values for i=4.
	// Total appended: 6+4+6+6+3 = 25.
	wantCount := 25
	wantLast := 42

	if !reflect.DeepEqual(result, wantCount) {
		t.Errorf("got result (len(found))=%v, want %v", result, wantCount)
	}

	if !reflect.DeepEqual(last, wantLast) {
		t.Errorf("got last=%v, want %v", last, wantLast)
	}
}
