package builtins

// Supplemental tests for the Length() and SizeOf() builtins (builtins/length.go).
//
// The primary tests live in builtins/utility_test.go (pre-existing).
// This file adds coverage for the channel, error, package, and extension paths.

import (
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Test_Length_NilArgReturnsZero verifies that nil returns 0 (not an error).
// The function has an early-exit guard for nil before any type switch.
func Test_Length_NilArgReturnsZero(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList(nil)

	got, err := Length(s, args)
	if err != nil {
		t.Fatalf("Length(nil) unexpected error: %v", err)
	}

	if got != 0 {
		t.Errorf("Length(nil) = %v, want 0", got)
	}
}

// Test_Length_ErrorTypeReturnsMessageLength verifies that len(error) returns
// the number of characters in the error message string.
func Test_Length_ErrorTypeReturnsMessageLength(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	// Create an Ego error whose string representation we know.
	e := errors.ErrDivisionByZero // or any error with a known message
	args := data.NewList(e)

	got, err := Length(s, args)
	if err != nil {
		t.Fatalf("Length(error) unexpected error: %v", err)
	}

	// The length should equal len(e.Error()).
	want := len(e.Error())
	if got != want {
		t.Errorf("Length(error) = %v, want %d", got, want)
	}
}

// Test_Length_MapReturnsKeyCount verifies that len(map) returns the number
// of keys in the map.
func Test_Length_MapReturnsKeyCount(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	m := data.NewMapFromMap(map[string]any{
		"a": 1,
		"b": 2,
		"c": 3,
	})
	args := data.NewList(m)

	got, err := Length(s, args)
	if err != nil {
		t.Fatalf("Length(map) unexpected error: %v", err)
	}

	if got != 3 {
		t.Errorf("Length(map) = %v, want 3", got)
	}
}

// Test_Length_PackageTypeReturnsError verifies that Length rejects a *data.Package
// argument with ErrInvalidType.  Packages do not have a meaningful length.
func Test_Length_PackageTypeReturnsError(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	pkg := &data.Package{}
	args := data.NewList(pkg)

	_, err := Length(s, args)
	if err == nil {
		t.Fatal("Length(*data.Package) expected error, got nil")
	}

	if !errors.Equals(errors.New(err), errors.ErrInvalidType) {
		t.Errorf("Length(*data.Package) error = %v, want ErrInvalidType", err)
	}
}

// Test_Length_ChannelEmptyOpenReturnsZero verifies that an open channel with no
// buffered items returns 0, matching Go's len(channel) semantics.
//
// BUILTIN-LENGTH-1 is resolved: Length() now calls arg.Len() which returns the
// actual buffered item count instead of the former math.MaxInt32 sentinel.
func Test_Length_ChannelEmptyOpenReturnsZero(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	ch := data.NewChannel(5) // capacity 5, but nothing sent yet
	args := data.NewList(ch)

	got, err := Length(s, args)
	if err != nil {
		t.Fatalf("Length(open empty channel) unexpected error: %v", err)
	}

	// An open channel with no buffered items has len() == 0.
	if got != 0 {
		t.Errorf("Length(open empty channel) = %v, want 0", got)
	}
}

// Test_Length_ChannelWithItemsReturnsCount verifies that an open channel with
// buffered items returns the actual count, not math.MaxInt32.
func Test_Length_ChannelWithItemsReturnsCount(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	ch := data.NewChannel(5)

	// Buffer two items.
	_ = ch.Send("first")
	_ = ch.Send("second")

	args := data.NewList(ch)

	got, err := Length(s, args)
	if err != nil {
		t.Fatalf("Length(channel with items) unexpected error: %v", err)
	}

	if got != 2 {
		t.Errorf("Length(channel with 2 items) = %v, want 2", got)
	}
}

// Test_Length_ChannelEmptyAfterCloseReturnsZero verifies that a channel that
// has been closed AND drained returns 0.
func Test_Length_ChannelEmptyAfterCloseReturnsZero(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	ch := data.NewChannel(2)
	// Close the channel so it is drained and closed.
	ch.Close()
	args := data.NewList(ch)

	got, err := Length(s, args)
	if err != nil {
		t.Fatalf("Length(closed+empty channel) unexpected error: %v", err)
	}

	if got != 0 {
		t.Errorf("Length(closed+empty channel) = %v, want 0", got)
	}
}

// Test_Length_StrictModeRejectsNumericArg verifies that a numeric argument
// returns an error in strict type-checking mode.
//
// In strict mode (typeChecking == StrictTypeEnforcement = 0), the default
// branch does not fall through to string conversion.
func Test_Length_StrictModeRejectsNumericArg(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	s.Root().SetAlways(defs.ExtensionsVariable, true)
	s.Root().SetAlways(defs.TypeCheckingVariable, defs.StrictTypeEnforcement) // 0

	args := data.NewList(42)

	_, err := Length(s, args)
	if err == nil {
		t.Fatal("Length(int) in strict mode expected error, got nil")
	}
}

// Test_Length_RelaxedModeAllowsNumericArg verifies that a numeric argument
// returns its string-formatted length in relaxed (non-strict) mode when
// extensions are enabled.
//
// In relaxed mode (typeChecking == RelaxedTypeEnforcement = 1), the default
// branch converts the argument to a string and returns its length.
func Test_Length_RelaxedModeAllowsNumericArg(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	s.Root().SetAlways(defs.ExtensionsVariable, true)
	s.Root().SetAlways(defs.TypeCheckingVariable, defs.RelaxedTypeEnforcement) // 1

	// "3.14" has 4 characters.
	args := data.NewList(3.14)
	
	got, err := Length(s, args)
	if err != nil {
		t.Fatalf("Length(float64) in relaxed mode error: %v", err)
	}

	// len("3.14") = 4
	if got != 4 {
		t.Errorf("Length(3.14) in relaxed mode = %v, want 4", got)
	}
}

// ---- SizeOf ----

// Test_SizeOf_NonNilValueReturnsPositiveSize verifies that SizeOf returns a
// positive number for any non-nil value.
func Test_SizeOf_NonNilValueReturnsPositiveSize(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList(12345)

	got, err := SizeOf(s, args)
	if err != nil {
		t.Fatalf("SizeOf(int) unexpected error: %v", err)
	}

	size, ok := got.(int)
	if !ok {
		t.Fatalf("SizeOf returned %T, want int", got)
	}

	if size <= 0 {
		t.Errorf("SizeOf(int) = %d, want > 0", size)
	}
}
