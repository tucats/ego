package bytecode

// Regression tests for the GORTNS-1 fix: RunFromAddress used to leak one
// goroutine per call because signal.Stop does not close the channel the SIGINT
// watcher was blocked on.
//
// These tests count goroutines rather than asserting on internal state, because
// "the goroutine went away" is the whole property under test. runtime.GC plus a
// short sleep gives the runtime a chance to finish reclaiming goroutines that
// have returned but not yet been cleaned up, which keeps the counts stable.

import (
	"runtime"
	"testing"
	"time"

	"github.com/tucats/ego/internal/language/symbols"
)

// settledGoroutineCount returns the number of live goroutines after giving the
// runtime a moment to reclaim any that have just finished.
func settledGoroutineCount() int {
	runtime.GC()
	time.Sleep(250 * time.Millisecond)

	return runtime.NumGoroutine()
}

// trivialBytecode builds the smallest sealed bytecode that runs to completion.
func trivialBytecode(t *testing.T) (*ByteCode, *symbols.SymbolTable) {
	t.Helper()

	b := New("goroutine leak probe")
	b.Emit(Push, 1)
	b.Emit(Push, 2)
	b.Emit(Add)
	b.Emit(Stop)
	b.Seal()

	return b, symbols.NewSymbolTable("goroutine leak probe")
}

// TestRunDoesNotLeakGoroutinePerContext covers the server's access pattern: a
// fresh Context per execution, as happens once per HTTP service request.
func TestRunDoesNotLeakGoroutinePerContext_GORTNS1(t *testing.T) {
	const runs = 200

	before := settledGoroutineCount()

	for i := 0; i < runs; i++ {
		b, s := trivialBytecode(t)

		if err := NewContext(s, b).Run(); err != nil {
			t.Fatalf("run %d failed: %v", i, err)
		}
	}

	after := settledGoroutineCount()

	// A small tolerance absorbs unrelated runtime bookkeeping goroutines. Before
	// the fix the delta was 201 for 200 runs, so any real leak is far outside it.
	if delta := after - before; delta > 5 {
		t.Errorf("leaked %d goroutines across %d Run calls (before=%d after=%d)",
			delta, runs, before, after)
	}
}

// TestRunDoesNotLeakGoroutinePerCall covers the amplified pattern: one Context
// run many times, which is what sort.Slice does with an Ego comparator (one
// Run per comparison) and what fmt does for an Ego String() method.
func TestRunDoesNotLeakGoroutinePerCall_GORTNS1(t *testing.T) {
	const calls = 2500

	b, s := trivialBytecode(t)
	ctx := NewContext(s, b)

	before := settledGoroutineCount()

	for i := 0; i < calls; i++ {
		if err := ctx.RunFromAddress(0); err != nil {
			t.Fatalf("call %d failed: %v", i, err)
		}
	}

	after := settledGoroutineCount()

	// Before the fix this leaked 2501 goroutines -- one per call.
	if delta := after - before; delta > 5 {
		t.Errorf("leaked %d goroutines across %d RunFromAddress calls (before=%d after=%d)",
			delta, calls, before, after)
	}
}
