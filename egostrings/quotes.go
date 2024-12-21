package egostrings

import (
	"encoding/json"
	"strings"
	"unicode"
)

// Escape escapes special characters in a string. This uses the
// JSON marshaller to create a suitable string value.
func Escape(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}

	// Trim the beginning and trailing " character
	return string(b[1 : len(b)-1])
}

// Unquote removes quotation marks from a string if present. This
// identifies any escaped quotes (preceded by a backslash) and removes
// the backslashes.
func Unquote(s string) string {
	if strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") {
		s = strings.TrimPrefix(strings.TrimSuffix(s, "\""), "\"")
	}

	return s
}

// HasCapitalizedName returns true if the first rune/character of the
// string is considered a capital letter in Unicode. This works even
// with Unicode characters that are not letters.
func HasCapitalizedName(name string) bool {
	var firstRune rune

	for _, ch := range name {
		firstRune = ch

		break
	}

	return unicode.IsUpper(firstRune)
}
