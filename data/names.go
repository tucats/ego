package data

import (
	"fmt"
	"sync/atomic"
)

var nameSequenceNumber int32 = 0

// Threadsafe name generator. This is used to create names such as temporary variables
// or scope block names during compilation and program execution.
func GenerateName() string {
	n := atomic.AddInt32(&nameSequenceNumber, 1)

	return fmt.Sprintf("$%d", n)
}
