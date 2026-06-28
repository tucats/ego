package oauth

import (
	"testing"
)

// TestIsJWT verifies that IsJWT correctly identifies compact-serialized JWT strings
// and rejects non-JWT strings such as Ego native hex tokens.
func TestIsJWT(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid three-segment JWT",
			input:    "eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyMSIsImV4cCI6OTk5OTk5OTk5OX0.signature",
			expected: true,
		},
		{
			name:     "minimal three-segment token",
			input:    "a.b.c",
			expected: true,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "one segment only",
			input:    "onlyone",
			expected: false,
		},
		{
			name:     "two segments",
			input:    "header.payload",
			expected: false,
		},
		{
			name:     "four segments",
			input:    "a.b.c.d",
			expected: false,
		},
		{
			name:     "empty first segment",
			input:    ".payload.signature",
			expected: false,
		},
		{
			name:     "empty second segment",
			input:    "header..signature",
			expected: false,
		},
		{
			name:     "empty third segment",
			input:    "header.payload.",
			expected: false,
		},
		{
			name:     "ego native hex token (no dots)",
			input:    "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
			expected: false,
		},
		{
			name:     "UUID-style string",
			input:    "550e8400-e29b-41d4-a716-446655440000",
			expected: false,
		},
		{
			name:     "only dots",
			input:    "..",
			expected: false,
		},
		{
			name:     "realistic JWT prefix eyJ",
			input:    "eyJhbGciOiJSUzI1NiJ9.eyJpc3MiOiJodHRwczovL2V4YW1wbGUuY29tIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsJWT(tt.input)
			if got != tt.expected {
				t.Errorf("IsJWT(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
