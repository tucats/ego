package data

import (
	"strconv"
	"sync/atomic"
)

// nameSequenceNumber is a process-wide counter used to generate unique names.
// It is declared as int32 so it can be incremented with sync/atomic operations,
// which are safe to call from multiple goroutines simultaneously without a mutex.
var nameSequenceNumber int32 = 0

// tempVariablePrefix is prepended to every generated name.  The "$" character
// is chosen because it is not a valid first character in an Ego or Go identifier,
// which guarantees that generated names can never clash with user-defined names.
const tempVariablePrefix = "$"

// GenerateName returns a new unique name of the form "$1", "$2", "$3", … on
// every call.  It is used to create temporary variable names and anonymous
// scope-block names during compilation and program execution.
//
// atomic.AddInt32 increments nameSequenceNumber and returns the new value in a
// single, indivisible CPU instruction.  This means that even if two goroutines
// call GenerateName at exactly the same moment, each will receive a different
// number — no mutex required.
func GenerateName() string {
	n := atomic.AddInt32(&nameSequenceNumber, 1)

	return tempVariablePrefix + strconv.Itoa(int(n))
}
