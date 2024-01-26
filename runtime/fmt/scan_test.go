package fmt

import "testing"

func Test_FindSpace(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		pos      int
		expected int
	}{
		{
			name:     "No spaces",
			data:     "abcdef",
			pos:      0,
			expected: 6,
		},
		{
			name:     "Spaces at the beginning",
			data:     "   abcdef",
			pos:      0,
			expected: 0,
		},
		{
			name:     "Spaces in the middle",
			data:     "abc   def",
			pos:      0,
			expected: 3,
		},
		{
			name:     "Spaces at the end",
			data:     "abcdef   ",
			pos:      0,
			expected: 6,
		},
		{
			name:     "Nested parentheses",
			data:     "abc(def )ghi",
			pos:      0,
			expected: 12,
		},
		{
			name:     "Nested braces",
			data:     "abc{d ef}ghi",
			pos:      0,
			expected: 12,
		},
		{
			name:     "Nested angle brackets",
			data:     "abc< def>ghi",
			pos:      0,
			expected: 12,
		},
		{
			name:     "Quoted strings",
			data:     `abc"de f"g hi`,
			pos:      0,
			expected: 10,
		},
		{
			name:     "Single quoted strings",
			data:     `ab c'd ef'g hi`,
			pos:      0,
			expected: 2,
		},
		{
			name:     "Mixed levels",
			data:     `abc(def{g hi} )jk l`,
			pos:      0,
			expected: 17,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findSpace(tt.data, tt.pos)
			if result != tt.expected {
				t.Errorf("findSpace(%q, %d) = %d, expected %d", tt.data, tt.pos, result, tt.expected)
			}
		})
	}
}

func Test_SkipSpaces(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		pos      int
		expected int
	}{
		{
			name:     "No spaces",
			data:     "abcdef",
			pos:      0,
			expected: 0,
		},
		{
			name:     "Spaces at the beginning",
			data:     "   abcdef",
			pos:      0,
			expected: 3,
		},
		{
			name:     "Spaces in the middle",
			data:     "abc   def",
			pos:      0,
			expected: 0,
		},
		{
			name:     "Spaces at the end",
			data:     "abcdef   ",
			pos:      0,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := skipSpaces(tt.data, tt.pos)
			if result != tt.expected {
				t.Errorf("skipSpaces(%q, %d) = %d, expected %d", tt.data, tt.pos, result, tt.expected)
			}
		})
	}
}
