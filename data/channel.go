package data

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

// Structure of an Ego channel wrapper around Go channels. In addition to
// a native channel object (of type interface{}), it includes a mutex to
// protect threaded access to the structure, as well as info about the
// size and state (open, closed, queue size) of the Ego channel.
type Channel struct {
	channel chan interface{}
	mutex   sync.RWMutex
	size    int
	isOpen  bool
	count   int
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
		count:   0,
		id:      uuid.New().String(),
		channel: make(chan interface{}, size),
	}

	ui.Log(ui.TraceLogger, "trace.chan.create",
		"name", c.String())

	return c
}

// Send transmits an arbitrary data object through the channel, if it
// is open. We must verify that the chanel is open before using it. It
// is important to put the logging message out brefore re-locking the
// channel since c.String needs a read-lock.
func (c *Channel) Send(datum interface{}) error {
	if c == nil {
		return errors.ErrNilPointerReference
	}

	if c.IsOpen() {
		if ui.IsActive(ui.TraceLogger) {
			ui.Log(ui.TraceLogger, "trace.chan.send",
				"name", c.String())
		}

		c.channel <- datum

		c.mutex.Lock()
		defer c.mutex.Unlock()

		c.count++

		return nil
	}

	return errors.ErrChannelNotOpen
}

// Receive accepts an arbitrary data object through the channel, waiting
// if there is no information available yet. If it's not open, we also
// check to see if the messages have all been drained by looking at the
// counter.
func (c *Channel) Receive() (interface{}, error) {
	if c == nil {
		return nil, errors.ErrNilPointerReference
	}

	ui.Log(ui.TraceLogger, "trace.chan.receive",
		"name", c.String())

	if !c.IsOpen() && c.count == 0 {
		return nil, errors.ErrChannelNotOpen
	}

	datum := <-c.channel

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.count--

	return datum, nil
}

// Return a boolean value indicating if this channel is still open for
// business.
func (c *Channel) IsOpen() bool {
	if c == nil {
		ui.Log(ui.InternalLogger, "runtime.chan.not.open")

		return false
	}

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.isOpen
}

// IsEmpty checks to see if a channel has been drained (i.e. it is
// closed and there are no more items). This is used by the len()
// function, for example.
func (c *Channel) IsEmpty() bool {
	if c == nil {
		ui.Log(ui.InternalLogger, "runtime.chan.not.open")

		return false
	}

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return !c.isOpen && c.count == 0
}

// Close the channel so no more sends are permitted to the channel, and
// the receiver can test for channel completion. Must do the logging
// before taking the exclusive lock so c.String() can work.
func (c *Channel) Close() bool {
	if c == nil {
		ui.Log(ui.InternalLogger, "runtime.chan.not.open")

		return false
	}

	if ui.IsActive(ui.TraceLogger) {
		ui.Log(ui.TraceLogger, "trace.chan.close",
			"name", c.String())
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	close(c.channel)

	wasActive := c.isOpen
	c.isOpen = false

	return wasActive
}

// Generate a human-readlable expression of a channel object. This is
// most often used for debugging, so it includes the UUID of the
// channel object so debugging a program with multiple channels is
// easier.
func (c *Channel) String() string {
	if c == nil {
		ui.Log(ui.InternalLogger, "runtime.chan.not.open")

		return "nil"
	}

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	state := "open"

	if !c.isOpen {
		state = "closed"
	}

	return fmt.Sprintf("chan(%s, size %d(%d), id %s)",
		state, c.size, c.count, c.id)
}
