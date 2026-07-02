package builtins

// Tests for the Close() builtin function (builtins/close.go).
//
// Close() implements the Ego built-in close() function, which can be applied
// to a channel or a struct that has a registered "Close" method.
//
// # BUG-29 regression coverage
//
// docs/ISSUES.md BUG-29 reported that closing an already-closed Ego channel
// crashed the entire ego process: data.Channel.Close() called Go's built-in
// close() unconditionally, which panics with "close of closed channel" on a
// second call, and nothing on that call path ever recovered from a panic.
//
// The fix changed data.Channel.Close() from `func() bool` to
// `func() (bool, error)`, and this file's Close() wrapper now returns that
// pair as a data.List so a double-close becomes a normal, catchable Ego
// error instead of a process-ending panic. Test_Close_DoubleCloseReturnsError
// below is the direct regression test for that fix.

import (
	"testing"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// asChannelCloseResult unpacks the data.List that Close() now returns for a
// *data.Channel argument into its two components (wasOpen bool, and the
// error, which is nil on success). Every test below that closes a channel
// uses this helper so the list-unpacking logic — and the failure messages
// if it's ever shaped wrong — only need to be written once.
func asChannelCloseResult(t *testing.T, got any) (wasOpen bool, closeErr error) {
	t.Helper()

	list, ok := got.(data.List)
	if !ok {
		t.Fatalf("Close(channel) returned %T, want data.List", got)
	}

	if list.Len() != 2 {
		t.Fatalf("Close(channel) returned a data.List of length %d, want 2", list.Len())
	}

	wasOpen, ok = list.Get(0).(bool)
	if !ok {
		t.Fatalf("Close(channel) list element 0 = %T, want bool", list.Get(0))
	}

	// The second element is nil on success. A type assertion on a nil
	// interface value would panic, so only assert to error when it is
	// actually non-nil.
	if e := list.Get(1); e != nil {
		closeErr, ok = e.(error)
		if !ok {
			t.Fatalf("Close(channel) list element 1 = %T, want error or nil", e)
		}
	}

	return wasOpen, closeErr
}

// Test_Close_ClosesChannel verifies that calling Close() on an open channel
// succeeds and actually closes the channel.
func Test_Close_ClosesChannel(t *testing.T) {
	// Create a buffered channel with capacity 5.
	ch := data.NewChannel(5)
	s := symbols.NewSymbolTable("test")
	args := data.NewList(ch)

	// Close() on a channel now returns a data.List of (wasOpen bool, err
	// error) — see the asChannelCloseResult helper and the package doc
	// comment above for why.
	got, err := Close(s, args)
	if err != nil {
		t.Fatalf("Close(channel) unexpected error: %v", err)
	}

	wasOpen, closeErr := asChannelCloseResult(t, got)

	if closeErr != nil {
		t.Errorf("Close(channel) unexpected inner error: %v", closeErr)
	}

	if !wasOpen {
		t.Error("Close(channel) returned false; channel should have been open")
	}
}

// Test_Close_ChannelCloseReturnsTrueWhenOpen verifies that closing an open
// channel for the first time reports wasOpen=true and no error.
func Test_Close_ChannelCloseReturnsTrueWhenOpen(t *testing.T) {
	ch := data.NewChannel(1)
	s := symbols.NewSymbolTable("test")
	args := data.NewList(ch)

	got, err := Close(s, args)
	if err != nil {
		t.Fatalf("Close(open channel) unexpected error: %v", err)
	}

	wasOpen, closeErr := asChannelCloseResult(t, got)

	if closeErr != nil {
		t.Errorf("Close(open channel) unexpected inner error: %v", closeErr)
	}

	// The channel was open when we closed it, so wasOpen must be true.
	if !wasOpen {
		t.Error("Close(open channel) returned false; channel should have been open")
	}
}

// Test_Close_DoubleCloseReturnsError is the direct regression test for
// BUG-29. It verifies that calling Close() a second time on the same
// channel:
//
//  1. does NOT panic (which is what used to crash the whole ego process),
//  2. returns wasOpen=false, and
//  3. returns a non-nil, catchable error (ErrChannelNotOpen) describing
//     the problem, instead of silently succeeding or aborting.
func Test_Close_DoubleCloseReturnsError(t *testing.T) {
	ch := data.NewChannel(1)
	s := symbols.NewSymbolTable("test")
	args := data.NewList(ch)

	// First close should succeed normally.
	got, err := Close(s, args)
	if err != nil {
		t.Fatalf("first Close(channel) unexpected error: %v", err)
	}

	wasOpen, closeErr := asChannelCloseResult(t, got)
	if !wasOpen || closeErr != nil {
		t.Fatalf("first Close(channel) = (wasOpen=%v, err=%v), want (true, nil)", wasOpen, closeErr)
	}

	// The second close is the case that used to panic and crash the whole
	// process (see BUG-29 in docs/ISSUES.md). Wrapping the call in its own
	// recover() lets this test report a clear failure message instead of
	// crashing the whole `go test` run if the fix ever regresses.
	var (
		panicked      bool
		secondGot     any
		secondCallErr error
	)

	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()

		secondGot, secondCallErr = Close(s, args)
	}()

	if panicked {
		t.Fatal("second Close(channel) panicked: BUG-29 fix was not applied")
	}

	if secondCallErr != nil {
		t.Fatalf("second Close(channel) unexpected outer error: %v", secondCallErr)
	}

	wasOpen, closeErr = asChannelCloseResult(t, secondGot)

	if wasOpen {
		t.Error("second Close(channel) reported wasOpen=true; channel was already closed")
	}

	if closeErr == nil {
		t.Fatal("second Close(channel) expected a non-nil error, got nil")
	}

	if !errors.Equals(errors.New(closeErr), errors.ErrChannelNotOpen) {
		t.Errorf("second Close(channel) error = %v, want ErrChannelNotOpen", closeErr)
	}
}

// Test_Close_InvalidTypeReturnsError verifies that calling Close() with an
// argument that is neither a channel nor a struct returns ErrInvalidType.
func Test_Close_InvalidTypeReturnsError(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	// An integer is not a closeable type.
	args := data.NewList(42)

	_, err := Close(s, args)
	if err == nil {
		t.Fatal("Close(int) expected error, got nil")
	}

	if !errors.Equals(errors.New(err), errors.ErrInvalidType) {
		t.Errorf("Close(int) error = %v, want ErrInvalidType", err)
	}
}

// Test_Close_StringArgumentReturnsError verifies that a string argument also
// triggers the invalid-type error path.
func Test_Close_StringArgumentReturnsError(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList("not-a-channel")

	_, err := Close(s, args)
	if err == nil {
		t.Fatal("Close(string) expected error, got nil")
	}

	if !errors.Equals(errors.New(err), errors.ErrInvalidType) {
		t.Errorf("Close(string) error = %v, want ErrInvalidType", err)
	}
}
