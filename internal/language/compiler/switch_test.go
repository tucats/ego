package compiler

import (
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
