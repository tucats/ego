package egostrings

import (
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// Gibberish converts a UUID into a compact, human-friendly string of
// lowercase letters and digits.
//
// A standard UUID looks like "550e8400-e29b-41d4-a716-446655440000" — 36
// characters including hyphens.  Gibberish encodes the same 128 bits of
// information in a shorter string that is easier to read and type.
//
// The alphabet used is:
//
//	abcdefghijkmnpqrstuvwxyz23456789  (32 characters, base 32)
//
// The letters "o" and "l" are deliberately omitted to avoid visual confusion
// with the digits "0" and "1".  The digits "0" and "1" are likewise omitted
// for the same reason.
//
// Algorithm:
//  1. Split the 16-byte UUID into two 64-bit unsigned integers (hi, low)
//     by shifting each byte into position.
//  2. Convert each integer to base-32 by repeatedly taking (value mod 32)
//     as the next digit index and dividing by 32.  This produces the digits
//     in least-significant-first order, which is fine for an opaque
//     identifier.
//  3. Concatenate the digits for low followed by the digits for hi.
func Gibberish(u uuid.UUID) string {
	var result strings.Builder

	// digits is the 32-character alphabet used as "digit" values in base-32
	// encoding.  Its length determines the radix.
	digits := []byte("abcdefghijkmnpqrstuvwxyz23456789")

	radix := uint64(len(digits))

	// Build two 64-bit integers from the UUID's 16 bytes.
	// Each iteration shifts the accumulator left by 8 bits (one byte) and
	// ORs in the next byte, effectively building a big-endian integer.
	var hi, low uint64

	for i := 0; i < 8; i++ {
		hi = hi<<8 + uint64(u[i])
	}

	for i := 9; i < 16; i++ {
		low = low<<8 + uint64(u[i])
	}

	// Encode low in base 32 — repeatedly extract the least-significant
	// "digit" (low % radix) and shift right (low / radix).
	for low > 0 {
		result.WriteByte(digits[low%radix])
		low = low / radix
	}

	// Encode hi the same way and append after the low digits.
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

// TruncateMiddle shortens text to at most maxSize characters by replacing the
// middle section with "...".  The first four and last four characters are
// always preserved so the result is still recognizable.
//
// This is used when logging authentication tokens: the token is long enough
// to be unique but should not be written to logs in its entirety for security
// reasons.
//
// If maxSize is less than or equal to 10, it is clamped to 10 to ensure the
// four-character prefix and four-character suffix always fit.
func TruncateMiddle(text string, maxSize int) string {
	if maxSize <= 10 {
		maxSize = 10
	}

	if len(text) > maxSize {
		// Keep the first 4 characters, insert "...", then keep the last 4.
		// fmt.Sprintf is used purely for convenience — it is equivalent to
		// text[:4] + "..." + text[len(text)-4:].
		text = fmt.Sprintf("%s...%s", text[:4], text[len(text)-4:])
	}

	return text
}

// HashString returns the SHA-256 hash of s as a lowercase hexadecimal string.
// The result is always 64 characters long (32 bytes × 2 hex digits per byte).
//
// SHA-256 is a one-way cryptographic hash function: given the hash you cannot
// recover the original string, but given the original string you can always
// reproduce the same hash.  This property makes it suitable for storing
// passwords — the server stores only the hash and compares hashes at login
// rather than storing the plaintext password.
//
// Implementation notes:
//   - sha256.New() creates a hash.Hash that accumulates bytes via Write.
//   - h.Sum(nil) finalizes the hash and returns the 32-byte digest.
//   - Each byte is formatted as two lowercase hex digits; if the byte value
//     is less than 0x10 (e.g. 0x0a) strconv.FormatInt would produce a
//     single character ("a"), so a leading "0" is prepended to keep every
//     byte exactly two hex characters wide.
func HashString(s string) string {
	var r strings.Builder

	// Create a new SHA-256 hasher and feed the input string to it.
	// The blank identifier _ discards the byte-count return value of Write,
	// which is always len([]byte(s)) for an in-memory hash and never errors.
	h := sha256.New()
	_, _ = h.Write([]byte(s))

	// Sum(nil) appends the current hash to a nil slice and returns the result.
	// Passing nil (rather than an existing slice) is idiomatic when you just
	// want the hash bytes themselves.
	v := h.Sum(nil)
	for _, b := range v {
		byteString := strconv.FormatInt(int64(b), 16)
		if len(byteString) < 2 {
			byteString = "0" + byteString
		}

		r.WriteString(byteString)
	}

	return r.String()
}
