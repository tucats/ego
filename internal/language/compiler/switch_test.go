package compiler

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// runWithTimeout runs an Ego program fragment with RunString(), but on a
// background goroutine so the test can bail out instead of hanging forever.
//
// This helper exists specifically because of BUG-25: before the fix, a
// "fallthrough" into a switch's "default" clause (or a "fallthrough" used as
// the very last statement of a switch, with nothing following it) compiled
// into a branch instruction that pointed at bytecode address 0. Running that
// program did not error out -- it silently jumped back to the start of the
// function and re-executed the switch, forever. If a future change
// accidentally reintroduces that bug, the buggy program would hang the
// goroutine below indefinitely rather than returning an error, which would
// otherwise hang this entire test binary (and, in turn, CI) until it was
// killed. Wrapping the call in a timeout turns that hang into a normal,
// readable test failure.
func runWithTimeout(t *testing.T, name string, s *symbols.SymbolTable, program string, timeout time.Duration) error {
	t.Helper()

	// errCh is buffered so the goroutine can always send its result and exit
	// on its own, even if this function has already returned because of a
	// timeout below. An unbuffered channel would leak the goroutine forever
	// in the timeout case, since nothing would ever be left to receive from it.
	errCh := make(chan error, 1)

	go func() {
		errCh <- RunString(name, s, program)
	}()

	select {
	case err := <-errCh:
		return err
	case <-time.After(timeout):
		t.Fatalf("test %q did not complete within %s; this looks like a regression of BUG-25 (fallthrough infinite loop)", name, timeout)

		return nil // unreachable, but required by the compiler
	}
}

// TestBUG25FallthroughIntoDefaultValueSwitch verifies that "fallthrough" from
// a matched "case" clause into the switch's "default" clause runs the default
// body exactly once, rather than looping forever (BUG-25).
func TestBUG25FallthroughIntoDefaultValueSwitch(t *testing.T) {
	program := `
		x := 2
		result := ""

		switch x {
		case 1:
			result = "one"
		case 2:
			result = "two"
			fallthrough
		default:
			result = result + "+default"
		}
	`

	s := symbols.NewRootSymbolTable(t.Name())

	if err := runWithTimeout(t, t.Name(), s, program, 5*time.Second); !errors.Nil(err) {
		t.Fatalf("unexpected error: %v", err)
	}

	result, found := s.Get("result")
	if !found || !reflect.DeepEqual(result, "two+default") {
		t.Errorf("got %v (%T), want %q", result, result, "two+default")
	}
}

// TestBUG25FallthroughIntoDefaultConditionalSwitch is the same scenario as
// above but for a conditional (expression-less) switch, since "fallthrough"
// is compiled by the same shared code path (compileSwitchCase) regardless of
// whether the switch has a value or is condition-based.
func TestBUG25FallthroughIntoDefaultConditionalSwitch(t *testing.T) {
	program := `
		count := 0

		switch {
		case count < 0:
			count = 100
		case count == 0:
			count++
			fallthrough
		default:
			count++
		}
	`

	s := symbols.NewRootSymbolTable(t.Name())

	if err := runWithTimeout(t, t.Name(), s, program, 5*time.Second); !errors.Nil(err) {
		t.Fatalf("unexpected error: %v", err)
	}

	count, found := s.Get("count")
	if !found || !reflect.DeepEqual(count, 2) {
		t.Errorf("got %v (%T), want %v", count, count, 2)
	}
}

// TestBUG25FallthroughAsFinalStatement verifies that "fallthrough" used as
// the last statement of a switch's last clause, with no case or default
// clause following it, is now rejected at compile time (matching real Go's
// "cannot fallthrough final case in switch" behavior) instead of compiling
// into a branch to bytecode address 0 (which caused an infinite loop).
func TestBUG25FallthroughAsFinalStatement(t *testing.T) {
	program := `
		switch 1 {
		case 1:
			fallthrough
		}
	`

	_, err := CompileString(t.Name(), program)
	if errors.Nil(err) {
		t.Fatalf("expected a compile error, but compilation succeeded")
	}

	if !errors.Equals(err, errors.ErrInvalidFallthrough) {
		t.Errorf("got error %v, want an ErrInvalidFallthrough error", err)
	}
}

// TestBUG25FallthroughAfterDefaultDeclaredEarlier covers a subtler variant of
// the same bug: the "default:" clause appears *before* the case that ends in
// "fallthrough", so a default block does exist somewhere in the switch, but
// it does not textually follow the fallthrough. Since nothing follows the
// fallthrough in source order, this must still be a compile error, exactly
// as if there were no default clause in the switch at all.
func TestBUG25FallthroughAfterDefaultDeclaredEarlier(t *testing.T) {
	program := `
		switch 1 {
		default:
			// intentionally empty
		case 1:
			fallthrough
		}
	`

	_, err := CompileString(t.Name(), program)
	if errors.Nil(err) {
		t.Fatalf("expected a compile error, but compilation succeeded")
	}

	if !errors.Equals(err, errors.ErrInvalidFallthrough) {
		t.Errorf("got error %v, want an ErrInvalidFallthrough error", err)
	}
}

// TestBUG25FallthroughIntoNextCaseStillWorks is a non-regression check: the
// already-correct case-to-case "fallthrough" path (fixed independently of
// this bug) must keep working after the BUG-25 changes to compileSwitch().
func TestBUG25FallthroughIntoNextCaseStillWorks(t *testing.T) {
	program := `
		log := ""

		switch 1 {
		case 1:
			log = log + "A"
			fallthrough
		case 2:
			log = log + "B"
		case 3:
			log = log + "C"
		}
	`

	s := symbols.NewRootSymbolTable(t.Name())

	if err := runWithTimeout(t, t.Name(), s, program, 5*time.Second); !errors.Nil(err) {
		t.Fatalf("unexpected error: %v", err)
	}

	log, found := s.Get("log")
	if !found || !reflect.DeepEqual(log, "AB") {
		t.Errorf("got %v (%T), want %q", log, log, "AB")
	}
}

// ---------------------------------------------------------------------------
// Fix BUG-31: a bare "break" inside a "switch" incorrectly targeted the
// enclosing "for" loop instead of the "switch" itself, and a "switch" with
// no enclosing loop at all rejected "break" as a compile error even though
// that is perfectly legal Go. The tests below exercise the fix from several
// angles: a plain break inside a switch inside a loop, a switch with no
// loop around it at all, "continue" reaching past a switch to the loop it
// is nested in, labeled break reaching past a switch to a labeled loop, and
// two switches nested inside each other.
// ---------------------------------------------------------------------------

// TestBUG31BreakInsideSwitchDoesNotExitEnclosingLoop is the primary
// reproducer from the bug report: a "break" inside a "switch" case must end
// only the switch, so the enclosing "for" loop keeps running its remaining
// iterations. Before the fix, this program would print only "0 1 2" because
// the mis-targeted break also exited the loop; the correct behavior (and
// what real Go does) is to run every iteration.
func TestBUG31BreakInsideSwitchDoesNotExitEnclosingLoop(t *testing.T) {
	program := `
		var visited []int

		for i := 0; i < 5; i++ {
			switch i {
			case 3:
				break
			}
			visited = append(visited, i)
		}
	`

	// AddStandard registers the runtime's built-in functions into the symbol
	// table, including the internal "$new" helper that the compiler emits
	// for "var visited []int" (a zero-value slice declaration). Without this
	// call, running the program below fails with "unknown identifier: $new"
	// even though the BUG-31 fix itself has nothing to do with "$new" - it
	// is simply a prerequisite for any test program that declares a "var"
	// of slice/struct type at the compiler-test level.
	s := symbols.NewRootSymbolTable(t.Name())
	AddStandard(s)

	if err := runWithTimeout(t, t.Name(), s, program, 5*time.Second); !errors.Nil(err) {
		t.Fatalf("unexpected error: %v", err)
	}

	visited, found := s.Get("visited")
	if !found {
		t.Fatalf("expected 'visited' variable to be set")
	}

	// The loop must have completed all five iterations (0 through 4); if
	// the bug was still present, the break would have escaped the loop early
	// after i==3 was appended, leaving "visited" with only 4 elements.
	got := fmt.Sprintf("%v", visited)
	want := "[0, 1, 2, 3, 4]"

	if got != want {
		t.Errorf("got visited=%s, want %s (break inside switch appears to have exited the enclosing loop)", got, want)
	}
}

// TestBUG31BreakInsideSwitchWithNoEnclosingLoop covers the bug report's
// second reproducer: a "switch" with no "for" loop wrapped around it at all.
// Before the fix, the compiler had no loop-stack entry for "break" to attach
// to in this situation and rejected the program with
// errors.ErrInvalidLoopControl ("loop control statement outside of
// for-loop"), even though a bare "break" ending a switch clause early is
// ordinary, legal Go with no loop involved whatsoever.
func TestBUG31BreakInsideSwitchWithNoEnclosingLoop(t *testing.T) {
	program := `
		x := 5
		reached := false

		switch x {
		case 5:
			break
		}

		reached = true
	`

	s := symbols.NewRootSymbolTable(t.Name())

	if err := runWithTimeout(t, t.Name(), s, program, 5*time.Second); !errors.Nil(err) {
		t.Fatalf("unexpected error compiling/running switch with no enclosing loop: %v", err)
	}

	reached, found := s.Get("reached")
	if !found || !reflect.DeepEqual(reached, true) {
		t.Errorf("got reached=%v (found=%v), want true", reached, found)
	}
}

// TestBUG31ContinueInsideSwitchReachesEnclosingLoop verifies the other half
// of the fix's design: unlike "break", a bare "continue" inside a switch has
// no switch-level meaning in Go at all, so it must skip past the switch's
// loop-stack entry and land on the nearest real "for" loop. If this ever
// regressed to attaching "continue" to the switch itself, the branch would
// have nowhere valid to land and the loop would not skip the current
// iteration as expected.
func TestBUG31ContinueInsideSwitchReachesEnclosingLoop(t *testing.T) {
	program := `
		var visited []int

		for i := 0; i < 5; i++ {
			switch i {
			case 2:
				continue
			}
			visited = append(visited, i)
		}
	`

	// See the comment on TestBUG31BreakInsideSwitchDoesNotExitEnclosingLoop:
	// AddStandard makes "$new" available for the "var visited []int" line.
	s := symbols.NewRootSymbolTable(t.Name())
	AddStandard(s)

	if err := runWithTimeout(t, t.Name(), s, program, 5*time.Second); !errors.Nil(err) {
		t.Fatalf("unexpected error: %v", err)
	}

	visited, found := s.Get("visited")
	if !found {
		t.Fatalf("expected 'visited' variable to be set")
	}

	// Iteration 2 is skipped by "continue" (it never reaches the append call
	// below the switch), but the loop itself keeps running to completion.
	got := fmt.Sprintf("%v", visited)
	want := "[0, 1, 3, 4]"

	if got != want {
		t.Errorf("got visited=%s, want %s (continue inside switch did not reach the enclosing loop)", got, want)
	}
}

// TestBUG31LabeledBreakFromNestedSwitchExitsLabeledLoop confirms that a
// labeled break (e.g. "break outer") still finds and exits the labeled
// enclosing "for" loop even when it is written inside a "switch" - the new
// switch loop-stack entry must not shadow or interfere with label lookup.
func TestBUG31LabeledBreakFromNestedSwitchExitsLabeledLoop(t *testing.T) {
	// The "outer: for" label syntax is only recognized by the compiler
	// while compiling inside a function body (see the functionDepth check
	// in statement.go), so the loop must live inside a function here rather
	// than as a bare top-level fragment like the other tests in this file.
	// The function returns the slice it built up so the test can still read
	// it back out of the root symbol table afterward.
	program := `
		func collect() []int {
			var visited []int

			outer:
			for i := 0; i < 5; i++ {
				switch i {
				case 2:
					break outer
				}
				visited = append(visited, i)
			}

			return visited
		}

		visited := collect()
	`

	// See the comment on TestBUG31BreakInsideSwitchDoesNotExitEnclosingLoop:
	// AddStandard makes "$new" available for the "var visited []int" line.
	s := symbols.NewRootSymbolTable(t.Name())
	AddStandard(s)

	if err := runWithTimeout(t, t.Name(), s, program, 5*time.Second); !errors.Nil(err) {
		t.Fatalf("unexpected error: %v", err)
	}

	visited, found := s.Get("visited")
	if !found {
		t.Fatalf("expected 'visited' variable to be set")
	}

	// "break outer" at i==2 must exit the "for" loop entirely (not just the
	// switch), so only iterations 0 and 1 ever reach the append call.
	got := fmt.Sprintf("%v", visited)
	want := "[0, 1]"

	if got != want {
		t.Errorf("got visited=%s, want %s (labeled break did not exit the outer loop)", got, want)
	}
}

// TestBUG31NestedSwitchBareBreakExitsOnlyInnerSwitch checks that when one
// switch is nested directly inside another (with no loop involved at all),
// an unlabeled "break" in the inner switch's case body exits only that
// inner switch. Since compileSwitch pushes one loop-stack entry per switch,
// the innermost one is always on top of the stack, so a bare break should
// naturally resolve to it without any special-casing.
func TestBUG31NestedSwitchBareBreakExitsOnlyInnerSwitch(t *testing.T) {
	program := `
		reachedAfterInner := false
		reachedAfterOuter := false

		switch 1 {
		case 1:
			switch true {
			case true:
				break
			}
			reachedAfterInner = true
		}

		reachedAfterOuter = true
	`

	s := symbols.NewRootSymbolTable(t.Name())

	if err := runWithTimeout(t, t.Name(), s, program, 5*time.Second); !errors.Nil(err) {
		t.Fatalf("unexpected error: %v", err)
	}

	reachedAfterInner, _ := s.Get("reachedAfterInner")
	reachedAfterOuter, _ := s.Get("reachedAfterOuter")

	if !reflect.DeepEqual(reachedAfterInner, true) {
		t.Errorf("got reachedAfterInner=%v, want true (break should only have exited the inner switch)", reachedAfterInner)
	}

	if !reflect.DeepEqual(reachedAfterOuter, true) {
		t.Errorf("got reachedAfterOuter=%v, want true", reachedAfterOuter)
	}
}

// TestBUG31ContinueInsideSwitchWithNoEnclosingLoopIsCompileError verifies
// that "continue" written inside a switch with no enclosing "for" loop is
// still correctly rejected. Real Go treats a bare "continue" outside of any
// for loop as a compile error, and that must remain true even though
// "break" in the same position is now legal (BUG-31 only changes what
// "break" targets, not "continue").
func TestBUG31ContinueInsideSwitchWithNoEnclosingLoopIsCompileError(t *testing.T) {
	program := `
		switch 1 {
		case 1:
			continue
		}
	`

	_, err := CompileString(t.Name(), program)
	if errors.Nil(err) {
		t.Fatalf("expected a compile error, but compilation succeeded")
	}

	if !errors.Equals(err, errors.ErrInvalidLoopControl) {
		t.Errorf("got error %v, want an ErrInvalidLoopControl error", err)
	}
}
