package egostrings

import (
	"encoding/json"
	"strings"
	"unicode"
)

// Escape returns a copy of s with special characters replaced by their JSON
// escape sequences — for example, a tab becomes `\t` and a double-quote
// becomes `\"`.
//
// The implementation delegates to json.Marshal, which produces a valid
// JSON string literal (including the surrounding double-quote characters).
// We then strip those outer quotes with a slice expression so the caller
// gets only the escaped content, not the enclosing quotes.
//
// Panicking on a Marshal error is intentional: json.Marshal only fails for
// values that contain invalid UTF-8 sequences or cyclic structures, neither
// of which can happen with a plain Go string.
func Escape(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}

	// b is a byte slice like `"hello\tworld"`.
	// b[1 : len(b)-1] trims the leading and trailing double-quote bytes.
	return string(b[1 : len(b)-1])
}

// Unquote removes a matching pair of double-quote characters from the start
// and end of s, if both are present.  If s is not double-quoted, it is
// returned unchanged.
//
// Note: this function does not process backslash escape sequences inside the
// string.  It is intended for stripping the outer quotes from already-parsed
// tokens, not for full JSON/Go string unquoting.  Use strconv.Unquote for
// that purpose.
func Unquote(s string) string {
	if strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") {
		// TrimSuffix removes the trailing `"`, then TrimPrefix removes the
		// leading `"`.  We chain the calls so both are done in one expression.
		s = strings.TrimPrefix(strings.TrimSuffix(s, "\""), "\"")
	}

	return s
}

// HasCapitalizedName reports whether the first Unicode character of name is an
// uppercase letter.
//
// In Go, iterating a string with "for _, ch := range s" decodes the string
// as UTF-8 and yields one rune (Unicode code point) per iteration.  We break
// after the first iteration so that only the first character is examined.
// unicode.IsUpper handles the full Unicode uppercase category, not just ASCII
// A-Z, so names beginning with accented capitals (e.g. "Ñoño") are correctly
// identified.
//
// This function is used to enforce Ego's visibility rule: identifiers that
// start with an uppercase letter are exported (public), while those starting
// with a lowercase letter are unexported (private).
func HasCapitalizedName(name string) bool {
	var firstRune rune

	for _, ch := range name {
		firstRune = ch

		break
	}

	return unicode.IsUpper(firstRune)
}

// SingleQuote wraps s in SQL-style single quotes, returning "'s'".
// This is a convenience helper used when constructing SQL query strings
// where string values must be enclosed in single quotes.
//
// Note: this function does not escape any single quotes that may already
// be present inside s.  If s comes from user input you should sanitize it
// first to prevent SQL injection.
func SingleQuote(s string) string {
	return "'" + s + "'"
}
