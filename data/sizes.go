package data

import (
	"github.com/DmitriyVTitov/size"
)

// SizeOf returns the size of the wrapped any item passed
// in as a parameter. For built-in types like int and string, this
// returns the native size (including getting the runtime-specific
// size of "int" for the running instance of Ego). For non-scalar
// types like maps and arrays, it returns the actual number of
// bytes needed to store the object.
func SizeOf(v any) int {
	return size.Of(v)
}
