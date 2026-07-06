package compiler

import (
	"testing"
	"time"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// TestBUG45UnrecoveredGoroutinePanicStopsMainProgram is the end-to-end
// regression test for BUG-45, using the exact reproducer from
// docs/ISSUES.md#BUG-45: a goroutine signals it has started via a channel,
// then panics without recovering. Before the fix, that panic printed a
// message but did not stop the main program, which went on to run a
// 50,000,000-iteration loop to completion (taking on the order of a minute
// on this interpreter) instead of aborting almost immediately.
//
// runWithTimeout (defined in switch_test.go, same package) bounds how long
// this test will wait: if the fix regresses, the loop actually runs to
// completion, which is slow but NOT an infinite hang, so a generous timeout
// both (a) comfortably exceeds how long the fixed behavior takes (well under
// a second: the panic aborts the loop before it starts) and (b) still fails
// this test promptly, well before the ~90+ seconds the loop would otherwise
// take, rather than waiting for it.
func TestBUG45UnrecoveredGoroutinePanicStopsMainProgram(t *testing.T) {
	program := `
		func worker(done chan) {
			done <- true
			panic("goroutine panic")
		}

		done := make(chan, 1)
		go worker(done)
		_ = <-done

		count := 0
		for i := 0; i < 50000000; i++ {
			count = count + 1
		}

		finished := true
	`

	s := symbols.NewRootSymbolTable(t.Name())

	err := runWithTimeout(t, t.Name(), s, program, 10*time.Second)

	if err == nil {
		t.Fatal("expected an error from the unrecovered goroutine panic stopping the program, got nil")
	}

	if errors.Equals(err, errors.ErrStop) {
		t.Error("BUG-45 regression: got errors.ErrStop -- the unrecovered goroutine panic " +
			"was not distinguished from the program ending normally")
	}

	// The strongest possible confirmation that the main program was actually
	// aborted, not merely that some error was eventually returned: "finished"
	// is only ever assigned AFTER the 50,000,000-iteration loop completes. If
	// it exists in the symbol table, the loop ran to completion and the fix
	// did not take effect.
	if _, found := s.Get("finished"); found {
		t.Error("BUG-45 regression: main program's loop ran to completion after an unrecovered goroutine panic")
	}
}

// TestBUG45OrdinaryGoroutineErrorStillStopsMainProgram is a regression guard
// for the behavior the original bug report used as its own point of
// comparison: an ordinary (non-panic) runtime error in a goroutine already
// correctly stopped the main program before this fix, and must continue to
// do so afterward.
func TestBUG45OrdinaryGoroutineErrorStillStopsMainProgram(t *testing.T) {
	program := `
		func worker(done chan) {
			done <- true
			x := 0
			y := 5 / x
			_ = y
		}

		done := make(chan, 1)
		go worker(done)
		_ = <-done

		count := 0
		for i := 0; i < 50000000; i++ {
			count = count + 1
		}

		finished := true
	`

	s := symbols.NewRootSymbolTable(t.Name())

	err := runWithTimeout(t, t.Name(), s, program, 10*time.Second)

	if err == nil {
		t.Fatal("expected an error from the goroutine's division by zero, got nil")
	}

	if _, found := s.Get("finished"); found {
		t.Error("main program's loop ran to completion after an ordinary goroutine error")
	}
}
