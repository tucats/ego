package dictionary

import (
	"sync/atomic"
)

// Dictionary is a shared map of substitutions that can be used across multiple tests.
// This maintains state for the duration of test operation, so values can be passed from
// one test result to another's test request definition, etc.
//
// Dictonary substutution is applied to URLs, request bodies, headers, parameters, and task
// parameters.
var Dictionary = make(map[string]string)

// This atomic integer counter is used to handle the "$seq" substitution which injects the next
// available sequence number starting with zero. This is typically used for substitution values
// that need to generate a unique identifier or value.
var sequence = atomic.Int32{}
