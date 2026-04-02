package egostrings

import (
	"strings"
	"unicode"
)

// JSONMinify accepts a JSON string and returns a new string with
// all unnecessary whitespace removed. It handles escaped quotes, etc.
func JSONMinify(input string) string {
	// Scan over the input string, removing all whitespace. If a character
	// is a double quote, ignore all text in the string including any escaped
	// quotes
	var (
		result   strings.Builder
		inQuotes bool
		escape   bool
	)

	for _, char := range input {
		if char == '\\' {
			escape = true
		}

		if char == '"' && !escape {
			inQuotes = !inQuotes
		} else if !inQuotes && unicode.IsSpace(char) {
			continue
		}

		result.WriteRune(char)

		if char != '\\' {
			escape = false
		}
	}

	return result.String()
}
