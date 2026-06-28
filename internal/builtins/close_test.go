package builtins

// Tests for the Close() builtin function (builtins/close.go).
//
// Close() implements the Ego built-in close() function, which can be applied
// to a channel or a struct that has a registered "Close" method.

import (
	"testing"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// Test_Close_ClosesChannel verifies that calling Close() on an open channel
// succeeds and actually closes the channel.
func Test_Close_ClosesChannel(t *testing.T) {
	// Create a buffered channel with capacity 5.
	ch := data.NewChannel(5)
	s := symbols.NewSymbolTable("test")
	args := data.NewList(ch)

	// Close() on a channel returns the result of ch.Close() which is a bool
	// indicating whether the channel was open (true) or already closed (false).
	got, err := Close(s, args)
	if err != nil {
		t.Fatalf("Close(channel) unexpected error: %v", err)
	}

	// The channel was open, so Close() should return true (was open).
	wasOpen, ok := got.(bool)
	if !ok {
		t.Fatalf("Close(channel) returned %T, want bool", got)
	}

	if !wasOpen {
		t.Error("Close(channel) returned false; channel should have been open")
	}
}

// Test_Close_ChannelCloseReturnsTrueWhenOpen verifies that a second call to
// Close() via the builtin panics (double-close is not guarded in data.Channel).
//
// NOTE: data.Channel.Close() calls Go's built-in close() on the underlying
// channel without checking whether it was already closed.  Calling Close()
// twice therefore panics.  This test documents that limitation by ensuring
// the FIRST close succeeds with wasOpen=true.
func Test_Close_ChannelCloseReturnsTrueWhenOpen(t *testing.T) {
	ch := data.NewChannel(1)
	s := symbols.NewSymbolTable("test")
	args := data.NewList(ch)

	got, err := Close(s, args)
	if err != nil {
		t.Fatalf("Close(open channel) unexpected error: %v", err)
	}

	wasOpen, ok := got.(bool)
	if !ok {
		t.Fatalf("Close(open channel) returned %T, want bool", got)
	}

	// The channel was open when we closed it, so wasOpen must be true.
	if !wasOpen {
		t.Error("Close(open channel) returned false; channel should have been open")
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
