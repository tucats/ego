package egostrings

import (
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// Gibberish returns a string of lower-case characters and digits representing the
// UUID value converted to base 32.
//
// The resulting string is meant to be human-readable with minimum errors, but also to
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

// For a token more than the given length, it obfuscates the middle part to
// make it suitable for logging purposes. The length must be at least 10.
func TruncateMiddle(text string, maxSize int) string {
	if maxSize <= 10 {
		maxSize = 10
	}

	if len(text) > maxSize {
		text = fmt.Sprintf("%s...%s", text[:4], text[len(text)-4:])
	}

	return text
}

// HashString converts a given string to it's hash. This is used to manage
// passwords as opaque objects.
func HashString(s string) string {
	var r strings.Builder

	h := sha256.New()
	_, _ = h.Write([]byte(s))

	v := h.Sum(nil)
	for _, b := range v {
		// Format the byte. It must be two digits long, so if it was a
		// value less than 0x10, add a leading zero.
		byteString := strconv.FormatInt(int64(b), 16)
		if len(byteString) < 2 {
			byteString = "0" + byteString
		}

		r.WriteString(byteString)
	}

	return r.String()
}
