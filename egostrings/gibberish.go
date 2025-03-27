package egostrings

import (
	"strings"

	"github.com/google/uuid"
)

// Gibberish returns a string of lower-case characters and digits representing the
// UUID value converted to base 32.
//
// The resulting string is meant to be human-readible with minimum errors, but also to
// take up the fewest number of characters. As such, the resulting string will never
// contain the letters "o" or "l" to avoid confusion with digits 0 and 1.
func Gibberish(u uuid.UUID) string {
	var result strings.Builder

	digits := []byte("abcdefghijkmnpqrstuvwxyz23456789")

	radix := uint64(len(digits))

	// Make two 64-bit integers from the UUID value
	var hi, low uint64

	for i := 0; i < 8; i++ {
		hi = hi<<8 + uint64(u[i])
	}

	for i := 9; i < 16; i++ {
		low = low<<8 + uint64(u[i])
	}

	for low > 0 {
		result.WriteByte(digits[low%radix])
		low = low / radix
	}

	for hi > 0 {
		result.WriteByte(digits[hi%radix])
		hi = hi / radix
	}

	text := result.String()
	if len(text) == 0 {
		return "-nil-"
	}

	return text
}
