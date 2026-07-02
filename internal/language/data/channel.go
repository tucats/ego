package data

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/errors"
)

// Structure of an Ego channel wrapper around Go channels. In addition to
// a native channel object (of type any), it includes a mutex to
// protect threaded access to the structure, as well as info about the
// size and state (open, closed, queue size) of the Ego channel.
type Channel struct {
	channel chan any
	mutex   sync.RWMutex
	size    int
	isOpen  bool
	id      string
}

// Create a mew instance of an Ego channel. The size passed indicates
// the buffer size, which is 1 unless size is greater than 1, in which
// case it is set to the given size.
func NewChannel(size int) *Channel {
	if size < 1 {
		size = 1
	}

	c := &Channel{
		isOpen:  true,
		size:    size,
		mutex:   sync.RWMutex{},
		id:      uuid.New().String(),
		channel: make(chan any, size),
	}

	ui.Log(ui.TraceLogger, "trace.chan.create", ui.A{
		"name": c.String()})

	return c
}

// Send transmits an arbitrary data object through the channel, if it
// is open. We must verify that the channel is open before using it. It
// is important to put the logging message out before re-locking the
// channel since c.String needs a read-lock.
//
// A deferred recover guards against the race between the IsOpen check
// and the actual send: if another goroutine closes the channel in that
// window, the send would panic without the recovery.
func (c *Channel) Send(datum any) (err error) {
	if c == nil {
		return errors.ErrNilPointerReference
	}

	if !c.IsOpen() {
		return errors.ErrChannelNotOpen
	}

	if ui.IsActive(ui.TraceLogger) {
		ui.Log(ui.TraceLogger, "trace.chan.send", ui.A{
			"name": c.String()})
	}

	defer func() {
		if r := recover(); r != nil {
			err = errors.ErrChannelNotOpen
		}
	}()

	c.channel <- datum

	return nil
}

// Receive accepts an arbitrary data object through the channel, waiting
// if there is no information available yet. If it's not open, we also
// check to see if the messages have all been drained by looking at the
// channel length.
func (c *Channel) Receive() (any, error) {
	if c == nil {
		return nil, errors.ErrNilPointerReference
	}

	ui.Log(ui.TraceLogger, "trace.chan.receive", ui.A{
		"name": c.String()})

	// Read isOpen under the lock so we get a consistent view. len() on
	// a channel is safe to call concurrently without a lock.
	c.mutex.RLock()
	open := c.isOpen
	c.mutex.RUnlock()

	if !open && len(c.channel) == 0 {
		return nil, errors.ErrChannelNotOpen
	}

	datum, ok := <-c.channel
	if !ok {
		return nil, errors.ErrChannelNotOpen
	}

	return datum, nil
}

// Return a boolean value indicating if this channel is still open for
// business.
func (c *Channel) IsOpen() bool {
	if c == nil {
		ui.Log(ui.InternalLogger, "runtime.chan.not.open", nil)

		return false
	}

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.isOpen
}

// Len returns the number of items currently buffered in the channel.
// This mirrors Go's built-in len(channel) and is used by the Ego len()
// function to report the actual queue depth rather than a sentinel value.
// The result is a snapshot — by the time the caller reads it, the channel
// may have gained or lost items due to concurrent sends or receives.
func (c *Channel) Len() int {
	if c == nil {
		return 0
	}

	// len() on a Go channel is safe to call without holding a lock.
	return len(c.channel)
}

// Cap returns the buffer capacity the channel was created with.
// This is used by NewInstanceOf (builtins/new.go) to create a new independent
// channel of the same size when $new(ch) is called.
func (c *Channel) Cap() int {
	if c == nil {
		return 0
	}

	// c.size is set once at construction and never mutated, so no lock needed.
	return c.size
}

// IsEmpty checks to see if a channel has been drained (i.e. it is
// closed and there are no more items). This is used by the len()
// function, for example.
func (c *Channel) IsEmpty() bool {
	if c == nil {
		ui.Log(ui.InternalLogger, "runtime.chan.not.open", nil)

		return false
	}

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return !c.isOpen && len(c.channel) == 0
}

// Close the channel so no more sends are permitted to the channel, and
// the receiver can test for channel completion. Must do the logging
// before taking the exclusive lock so c.String() can work.
//
// wasOpen reports whether the channel was open (and is therefore being
// newly closed by this call). If the channel was already closed, Close
// returns a non-nil error instead of touching the channel again.
//
// # Why the isOpen check below matters (background for new Go developers)
//
// Go's built-in close() function panics if you call it on a channel that
// has already been closed — the exact panic message is
// "close of closed channel". A panic that is never recovered crashes the
// whole program, which for Ego means crashing the entire `ego` process,
// not just the Ego script that triggered it.
//
// An earlier version of this function called Go's close() unconditionally,
// every time Close() was called, with no check first. That meant a second
// call to Close() on the same channel — an easy mistake to make, and one
// real Ego programs were hitting — brought down the whole interpreter (see
// BUG-29 in docs/ISSUES.md for the original bug report and repro).
//
// The fix is straightforward: check c.isOpen BEFORE calling the native
// close(), and only call it when the channel is actually still open. Since
// that check and the close() call both happen while c.mutex is held
// exclusively (see the Lock()/Unlock() below), no other goroutine can slip
// in a second, concurrent Close() call between the check and the close —
// so close() can never be called twice, and the panic can never happen.
func (c *Channel) Close() (wasOpen bool, err error) {
	if c == nil {
		ui.Log(ui.InternalLogger, "runtime.chan.not.open", nil)

		return false, nil
	}

	if ui.IsActive(ui.TraceLogger) {
		ui.Log(ui.TraceLogger, "trace.chan.close", ui.A{
			"name": c.String()})
	}

	// Hold the exclusive (write) lock for the entire check-then-close
	// sequence below. "Exclusive" means no other goroutine can be holding
	// any lock (read or write) on this same mutex at the same time, which
	// is what makes the check-then-close sequence safe from a race with
	// another goroutine's concurrent Close() call.
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.isOpen {
		// Already closed by an earlier call. Report this as an ordinary,
		// catchable Ego error (the same one Send() already returns for a
		// closed channel) instead of calling the native close() again.
		return false, errors.ErrChannelNotOpen
	}

	close(c.channel)
	c.isOpen = false

	return true, nil
}

// Generate a human-readable expression of a channel object. This is
// most often used for debugging, so it includes the UUID of the
// channel object so debugging a program with multiple channels is
// easier.
func (c *Channel) String() string {
	if c == nil {
		ui.Log(ui.InternalLogger, "runtime.chan.not.open", nil)

		return "nil"
	}

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	state := "open"

	if !c.isOpen {
		state = "closed"
	}

	return fmt.Sprintf("chan(%s, size %d(%d), id %s)",
		state, c.size, len(c.channel), c.id)
}
