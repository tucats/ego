package data

import (
	"github.com/DmitriyVTitov/size"
)

// SizeOf returns the number of bytes required to store the given value.
//
// For built-in scalar types (int, float64, bool, etc.) the result is the
// fixed platform-native size — for example, int is 8 bytes on a 64-bit
// system.  For dynamically-sized types like strings, slices, maps, and
// Ego's own Array/Map/Struct wrappers, SizeOf walks the data structure
// recursively and sums the memory actually in use.
//
// The implementation delegates to the third-party "DmitriyVTitov/size"
// package, which uses Go's reflect package to inspect any value at runtime
// without needing type-specific code for every possible type.
func SizeOf(v any) int {
	return size.Of(v)
}
