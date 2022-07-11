package datatypes

import (
	"fmt"
	"sync"
)

var nameSequenceNumber int = 0
var nameMutex sync.Mutex

// Threadsafe name generator.
func GenerateName() string {
	nameMutex.Lock()
	defer nameMutex.Unlock()

	nameSequenceNumber++
	n := nameSequenceNumber

	return fmt.Sprintf("$%d", n)
}
