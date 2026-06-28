package egostrings

import (
	"errors"
	"strconv"
	"strings"
)

// Atoi converts a string to an integer, extending the standard library's
// strconv.Atoi to understand several numeric literal formats that Ego source
// code may contain:
//
//   - Decimal:     "42", "-7"
//   - Hexadecimal: "0x1F", "0xFF"  (prefix "0x")
//   - Octal:       "0o17"          (prefix "0o")
//   - Binary:      "0b1010"        (prefix "0b")
//   - Rune literal: "'A'"          (a single character enclosed in single quotes)
//
// The input is trimmed of leading/trailing whitespace before parsing.
// Radix prefixes ("0x", "0X", "0o", "0O", "0b", "0B") are recognized in
// either case, but the rest of the string is not lowercased, so rune literals
// like "'A'" return 65 (not 97).
//
// If the string cannot be parsed, the function returns 0 and a non-nil error.
func Atoi(s string) (int, error) {
	var (
		err error
		v   int64
		vi  int
	)

	// Remove surrounding whitespace.  We do NOT lowercase the whole string
	// because that would corrupt rune literals: "'A'" would become "'a'" and
	// return 97 instead of the correct 65.
	s = strings.TrimSpace(s)

	// A string enclosed in single quotes is a rune literal (e.g. "'A'").
	// We extract the character between the quotes, convert it to a []rune
	// (which handles multi-byte UTF-8 characters correctly), and use its
	// Unicode code point as the integer value.
	if len(s) > 1 && s[0] == '\'' && s[len(s)-1] == '\'' {
		runes := []rune(s[1 : len(s)-1])
		if len(runes) != 1 {
			return 0, errors.New("error.invalid.rune")
		}

		v = int64(runes[0])
	} else if strings.HasPrefix(strings.ToLower(s[:min(len(s), 2)]), "0x") {
		// Hexadecimal — skip the "0x"/"0X" prefix and parse with base 16.
		// The 64-bit parse ensures we don't truncate large hex values on
		// 32-bit builds before converting to int.
		v, err = strconv.ParseInt(s[2:], 16, 64)
	} else if strings.HasPrefix(strings.ToLower(s[:min(len(s), 2)]), "0o") {
		// Octal — skip the "0o" prefix and parse with base 8.
		v, err = strconv.ParseInt(s[2:], 8, 64)
	} else if strings.HasPrefix(strings.ToLower(s[:min(len(s), 2)]), "0b") {
		// Binary — skip the "0b"/"0B" prefix and parse with base 2.
		v, err = strconv.ParseInt(s[2:], 2, 64)
	} else {
		// Plain decimal integer — delegate to the standard library.
		// strconv.Atoi returns int (not int64), so we widen it afterward.
		vi, err = strconv.Atoi(s)
		v = int64(vi)
	}

	// On any parse error reset v to 0 so the caller always gets a safe
	// zero value alongside the error rather than a partial result.
	if err != nil {
		v = 0
	}

	return int(v), err
}
