package egostrings

import (
	"strings"
	"unicode"
)

// JSONMinify removes all unnecessary whitespace from a JSON string without
// altering its meaning.  Whitespace inside quoted string values is preserved;
// only whitespace that exists between JSON tokens (braces, brackets, colons,
// commas, and string literals) is removed.
//
// For example:
//
//	{"name": "Alice", "age": 30}  →  {"name":"Alice","age":30}
//
// The function is used before writing JSON to the network so that responses
// are compact without sacrificing readability in source code (where the JSON
// is often pretty-printed for clarity).
func JSONMinify(input string) string {
	var (
		result   strings.Builder // accumulates the output characters
		inQuotes bool            // true while the scanner is inside a JSON string value
		escape   bool            // true for the character immediately after a backslash
	)

	for _, char := range input {
		// A backslash puts the scanner into "escape mode" for the very next
		// character.  This means the character after '\' is never treated as a
		// special JSON character (e.g. '\"' inside a string does not close the
		// string).
		if char == '\\' {
			escape = true
		}

		if char == '"' && !escape {
			// An unescaped double-quote toggles whether we are inside a string.
			// While inQuotes is true, whitespace is significant and must not be
			// stripped.
			inQuotes = !inQuotes
		} else if !inQuotes && unicode.IsSpace(char) {
			// We are outside a quoted string and the character is whitespace
			// (space, tab, newline, etc.) — skip it entirely.
			continue
		}

		// Write every character that was not skipped above.
		result.WriteRune(char)

		// Reset escape mode after any character that is not itself a backslash.
		// The sequence "\\" is two backslashes: the first sets escape=true, the
		// second is written and then resets it, correctly leaving escape=false.
		if char != '\\' {
			escape = false
		}
	}

	return result.String()
}
