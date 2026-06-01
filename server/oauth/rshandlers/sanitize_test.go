// Package rshandlers — sanitize_test.go tests the sanitizeLogValue helper used
// by CallbackHandler to prevent log injection from caller-supplied URL parameters
// (OAUTH-L4).
//
// This file uses "package rshandlers" (the internal test package) so it can
// access the unexported sanitizeLogValue function directly.
package rshandlers

import "testing"

// TestSanitizeLogValue verifies that sanitizeLogValue strips ASCII control
// characters and trims surrounding whitespace (OAUTH-L4).
//
// Log injection works by embedding newline characters in user-controlled input
// (such as URL query parameters) that is later written to a log file.  In a
// newline-delimited JSON log, a single injected \n splits one log entry into
// two, and the second part looks like a separate JSON object.  sanitizeLogValue
// prevents this by replacing any control character with a space.
func TestSanitizeLogValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Normal printable ASCII passes through unchanged.
		{"plain ASCII", "access_denied", "access_denied"},

		// Spaces are preserved — they are not control characters.
		{"spaces preserved", "invalid request", "invalid request"},

		// Newline is replaced with space (the primary log-injection vector).
		{"newline replaced", "line1\nline2", "line1 line2"},

		// Carriage-return is replaced.
		{"CR replaced", "line1\rline2", "line1 line2"},

		// CRLF pair — both characters are each replaced by one space.
		{"CRLF replaced", "line1\r\nline2", "line1  line2"},

		// Tab is a control character and must be replaced.
		{"tab replaced", "col1\tcol2", "col1 col2"},

		// Null byte replaced.
		{"null byte replaced", "abc\x00def", "abc def"},

		// DEL (U+007F) is a control character and must be replaced.
		{"DEL replaced", "abc\x7Fdef", "abc def"},

		// Leading and trailing whitespace is trimmed.
		{"trim surrounding spaces", "  hello  ", "hello"},

		// Leading/trailing newlines are stripped (via TrimSpace).
		{"trim surrounding newlines", "\nhello\n", "hello"},

		// Non-ASCII Unicode (emoji, accented letters, CJK) passes through — it
		// does not affect newline-delimited JSON formatting.
		{"unicode passes", "héllo wörld 🌍", "héllo wörld 🌍"},

		// Empty string stays empty.
		{"empty string", "", ""},

		// String consisting only of control chars becomes empty after trimming.
		{"only control chars", "\n\r\t", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeLogValue(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeLogValue(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
