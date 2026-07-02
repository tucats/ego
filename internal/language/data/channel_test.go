package data

// channel_test.go also contains regression tests for BUG-29 (see
// docs/ISSUES.md): closing an already-closed Channel used to call Go's
// built-in close() a second time, which panics with "close of closed
// channel" and crashed the entire ego process. Close() now checks whether
// the channel is already closed before ever calling the native close(),
// and reports that condition as an ordinary Go error instead.

import (
	"testing"

	"github.com/tucats/ego/internal/errors"
)

func TestNewChannel(t *testing.T) {
	fakeID := "49473e93-9f74-4c88-9234-5e037f2bac13"

	type args struct {
		size int
	}

	tests := []struct {
		name string
		args args
		want *Channel
	}{
		{
			name: "single item channel",
			args: args{size: 1},
			want: &Channel{
				size:   1,
				isOpen: true,
				id:     fakeID,
			},
		},
		{
			name: "multi item channel",
			args: args{size: 5},
			want: &Channel{
				size:   5,
				isOpen: true,
				id:     fakeID,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewChannel(tt.args.size)
			// Compare what we can.
			match := true
			if got.isOpen != tt.want.isOpen ||
				got.size != tt.want.size {
				match = false
			}

			if !match {
				t.Errorf("NewChannel() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test_Channel_Close_FirstCallSucceeds verifies that closing a freshly
// created (open) channel reports wasOpen=true and no error, and that the
// channel is actually closed afterward (IsOpen() becomes false).
func Test_Channel_Close_FirstCallSucceeds(t *testing.T) {
	c := NewChannel(1)

	wasOpen, err := c.Close()
	if err != nil {
		t.Fatalf("Close() unexpected error: %v", err)
	}

	if !wasOpen {
		t.Error("Close() returned wasOpen=false; channel should have been open")
	}

	if c.IsOpen() {
		t.Error("IsOpen() is still true after Close()")
	}
}

// Test_Channel_Close_DoubleCloseDoesNotPanic is the direct regression test
// for BUG-29 (docs/ISSUES.md). Before the fix, this exact sequence — Close()
// called twice on the same channel — panicked with "close of closed
// channel" because Close() called Go's native close() unconditionally on
// every call. A panic here, if the fix ever regresses, would otherwise
// crash this entire test binary; wrapping the second call in its own
// recover() turns that into an ordinary, readable test failure instead.
func Test_Channel_Close_DoubleCloseDoesNotPanic(t *testing.T) {
	c := NewChannel(1)

	// First close: must succeed.
	if _, err := c.Close(); err != nil {
		t.Fatalf("first Close() unexpected error: %v", err)
	}

	var (
		panicked  bool
		wasOpen   bool
		secondErr error
	)

	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()

		wasOpen, secondErr = c.Close()
	}()

	if panicked {
		t.Fatal("second Close() call panicked: BUG-29 fix was not applied")
	}

	if wasOpen {
		t.Error("second Close() reported wasOpen=true; channel was already closed")
	}

	if secondErr == nil {
		t.Fatal("second Close() expected a non-nil error, got nil")
	}

	if !errors.Equals(errors.New(secondErr), errors.ErrChannelNotOpen) {
		t.Errorf("second Close() error = %v, want ErrChannelNotOpen", secondErr)
	}
}

// Test_Channel_Close_NilReceiver verifies that calling Close() on a nil
// *Channel does not panic — it should behave the same as before this fix
// (report wasOpen=false, no error), since a nil receiver is a different,
// pre-existing situation from the double-close case BUG-29 was about.
func Test_Channel_Close_NilReceiver(t *testing.T) {
	var c *Channel

	wasOpen, err := c.Close()
	if err != nil {
		t.Errorf("Close() on nil receiver returned unexpected error: %v", err)
	}

	if wasOpen {
		t.Error("Close() on nil receiver returned wasOpen=true, want false")
	}
}
