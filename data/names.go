package data

import (
	"fmt"
	"sync/atomic"
)

var nameSequenceNumber int32 = 0

// Threadsafe name generator.
func GenerateName() string {
	n := atomic.AddInt32(&nameSequenceNumber, 1)

	return fmt.Sprintf("$%d", n)
}
