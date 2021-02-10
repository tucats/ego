package datatypes

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

// Structure of an Ego channel wrapper around Go channels.
type Channel struct {
	channel chan interface{}
	mutex   sync.Mutex
	size    int
	isOpen  bool
	count   int
	id      string
}

const (
	ChannelNotOpenError = "channel not open"
)

// Create a mew instance of an Ego channel. The size passed indicates
// the buffer size, which is 1 unless size is greater than 1, in which
// case it is set to the given size.
func NewChannel(size int) *Channel {
	if size < 1 {
		size = 1
	}

	c := &Channel{
		isOpen: true,
		size:   size,
		mutex:  sync.Mutex{},
		count:  0,
		id:     uuid.New().String(),
	}
	c.channel = make(chan interface{}, size)

	ui.Debug(ui.ByteCodeLogger, "--> Created  %s", c.String())

	return c
}

// Send transmits an arbitrary data object through the channel, if it
// is open.
func (c *Channel) Send(datum interface{}) *errors.EgoError {
	c.mutex.Lock()

	defer c.mutex.Unlock()

	if c.isOpen {
		ui.Debug(ui.ByteCodeLogger, "--> Sending on %s", c.String())

		c.count++
		c.channel <- datum

		return nil
	}

	return errors.New(errors.ChannelNotOpenError)
}

// Receive accepts an arbitrary data object through the channel, waiting
// if there is no information available yet. If it's not open, we also
// check to see if the messages have all been drained by looking at the
// counter.
func (c *Channel) Receive() (interface{}, *errors.EgoError) {
	ui.Debug(ui.ByteCodeLogger, "--> Receiving on %s", c.String())

	if !c.isOpen && c.count == 0 {
		return nil, errors.New(errors.ChannelNotOpenError)
	}

	datum := <-c.channel
	c.count--

	return datum, nil
}

// Return a boolean value indicating if this channel is still open for
// business.
func (c *Channel) IsOpen() bool {
	return c.isOpen
}

// IsEmpty checks to see if a channel has been drained (i.e. it is
// closed and there are no more items). This is used by the len()
// function, for example.
func (c *Channel) IsEmpty() bool {
	return !c.isOpen && c.count == 0
}

func (c *Channel) GetSize() int {
	return c.size
}

// Close the channel so no more sends are permitted to the channel, and
// the receiver can test for channel completion.
func (c *Channel) Close() bool {
	c.mutex.Lock()

	defer c.mutex.Unlock()

	ui.Debug(ui.ByteCodeLogger, "--> Closing %s", c.String())

	wasActive := c.isOpen

	close(c.channel)

	c.isOpen = false

	return wasActive
}

func (c *Channel) String() string {
	state := "open"

	if !c.isOpen {
		state = "closed"
	}

	return fmt.Sprintf("chan(%s, size %d(%d), id %s)",
		state, c.size, c.count, c.id)
}
