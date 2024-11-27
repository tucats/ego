package debugger

import (
	"strings"
	"testing"

	"github.com/tucats/ego/tokenizer"
)

func TestBreakCommand_InvalidClauses(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Missing subclause",
			input:    "break ",
			expected: "invalid break clause",
		},
		{
			name:     "Invalid subclause",
			input:    "break instead of 3 +",
			expected: "invalid break clause",
		},
		{
			name:     "Invalid 'when' clause",
			input:    "break when 3 +",
			expected: "unexpected token",
		},
		{
			name:     "Invalid 'when' clause with no condition",
			input:    "break when",
			expected: "unexpected token",
		},
		{
			name:     "Invalid 'when' clause with multiple conditions",
			input:    "break when * and -",
			expected: "unexpected token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := tokenizer.New(tt.input, true)

			err := breakCommand(tokenizer)
			if err != nil && tt.expected != "" && !strings.Contains(err.Error(), tt.expected) {
				t.Errorf("got error = %v, want error = %v", err, tt.expected)
			}

			if err == nil && tt.expected != "" {
				t.Errorf("got error = %v, want error = %v", err, tt.expected)
			}
		})
	}
}
