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

// ─────────────────────────────────────────────────────────────────────────────
// L5: permissionsToData correctness and efficiency
// ─────────────────────────────────────────────────────────────────────────────

// TestPermissionsToData_Empty verifies that an empty permission slice returns
// an empty string rather than a leading comma or panic (L5).
func TestPermissionsToData_Empty(t *testing.T) {
	got := permissionsToData(nil)
	if got != "" {
		t.Errorf("permissionsToData(nil) = %q, want empty string", got)
	}

	got = permissionsToData([]string{})
	if got != "" {
		t.Errorf("permissionsToData([]) = %q, want empty string", got)
	}
}

// TestPermissionsToData_Single verifies that a single-element slice produces
// just the element with no trailing comma (L5).
func TestPermissionsToData_Single(t *testing.T) {
	got := permissionsToData([]string{"ego.logon"})
	if got != "ego.logon" {
		t.Errorf("permissionsToData([logon]) = %q, want %q", got, "ego.logon")
	}
}

// TestPermissionsToData_Multiple verifies the comma-separated format for a
// typical multi-permission list (L5).
//
// Before the L5 fix, permissionsToData used string concatenation in a loop.
// In Go, string concatenation allocates a new backing array on every +=, so
// the total memory copied is O(1+2+…+n) = O(n²).  strings.Join performs a
// single allocation of exactly the right size.
//
// The output format must be identical: this test confirms the rewrite is a
// pure performance improvement with no behavioral change.
func TestPermissionsToData_Multiple(t *testing.T) {
	perms := []string{"ego.logon", "ego.tables.read", "ego.root"}
	want := "ego.logon,ego.tables.read,ego.root"

	got := permissionsToData(perms)
	if got != want {
		t.Errorf("permissionsToData(%v) = %q, want %q", perms, got, want)
	}
}

// TestPermissionsToData_NoLeadingOrTrailingComma verifies that
// permissionsToData never produces a string that starts or ends with a comma,
// which would create an empty field when the result is later split on commas
// (L5 correctness guard).
func TestPermissionsToData_NoLeadingOrTrailingComma(t *testing.T) {
	perms := []string{"ego.logon", "ego.root"}
	got := permissionsToData(perms)

	if len(got) > 0 && got[0] == ',' {
		t.Errorf("permissionsToData result starts with comma: %q", got)
	}

	if len(got) > 0 && got[len(got)-1] == ',' {
		t.Errorf("permissionsToData result ends with comma: %q", got)
	}
}
