package errors

import (
	"errors"
	"testing"
)

// Regression tests for docs/ISSUES.md BUG-66: In() and At() used to mutate
// the receiver in place and return the same pointer, unlike Context() (which
// already cloned). Any error value held by more than one variable -- most
// notably a package-level "sentinel" error like ErrDivisionByZero, but
// equally an ordinary error value copied into two Ego variables -- would
// have every holder's formatted message and location silently altered by a
// single call to In() or At() on any one of them.

// TestError_In_DoesNotMutateReceiver verifies that calling In() on an error
// leaves the original error's location name untouched, returning a distinct
// clone instead.
func TestError_In_DoesNotMutateReceiver(t *testing.T) {
	base := &Error{err: errors.New("test error")}

	derived := base.In("myFunc")

	if base.HasIn() {
		t.Errorf("base.In() call mutated the receiver: base.location = %+v", base.location)
	}

	if !derived.HasIn() || derived.location.name != "myFunc" {
		t.Errorf("derived error missing expected location name: %+v", derived.location)
	}

	if base == derived {
		t.Error("In() returned the same pointer as the receiver; expected a clone")
	}
}

// TestError_At_DoesNotMutateReceiver verifies that calling At() on an error
// leaves the original error's location line/column untouched, returning a
// distinct clone instead.
func TestError_At_DoesNotMutateReceiver(t *testing.T) {
	base := &Error{err: errors.New("test error")}

	derived := base.At(42, 7)

	if base.HasAt() {
		t.Errorf("base.At() call mutated the receiver: base.location = %+v", base.location)
	}

	if !derived.HasAt() || derived.location.line != 42 || derived.location.column != 7 {
		t.Errorf("derived error missing expected line/column: %+v", derived.location)
	}

	if base == derived {
		t.Error("At() returned the same pointer as the receiver; expected a clone")
	}
}

// TestError_In_SharedValue_TwoCallersDoNotInterfere reproduces the exact
// scenario found via Ego test code: a single error value referenced by two
// variables, where calling In() through one variable must not retroactively
// change what the other variable reports.
func TestError_In_SharedValue_TwoCallersDoNotInterfere(t *testing.T) {
	shared := Message("not found")

	e1 := shared.In("readFile")
	e2 := shared.In("writeFile")

	if e1.GetLocation() != "in readFile" {
		t.Errorf("e1 location = %q, want %q", e1.GetLocation(), "in readFile")
	}

	if e2.GetLocation() != "in writeFile" {
		t.Errorf("e2 location = %q, want %q", e2.GetLocation(), "in writeFile")
	}
}
