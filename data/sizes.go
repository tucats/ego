package data

import (
	"bytes"
	"encoding/gob"
	"math/bits"
)

// SizeOf returns the size of the wrapped interface{} item passed
// in as a parameter. For built-in types like int and string, this
// returns the native size (including getting the runtime-specific
// size of "int" for the running instance of Ego). For non-scalar
// types like maps and arrays, it returns the actual number of 
// bytes needed to store the object.
func SizeOf(v interface{}) int {
	size := 0

	switch v.(type) {
	case bool:
		size = 1
	case byte:
		size = 1
	case int32:
		size = 4
	case int:
		size = bits.UintSize
	case int64:
		size = 8
	case float32:
		size = 4
	case float64:
		size = 8
	default:
		b := new(bytes.Buffer)
		if err := gob.NewEncoder(b).Encode(v); err != nil {
			return 0
		}

		size = b.Len()
	}

	return size
}
