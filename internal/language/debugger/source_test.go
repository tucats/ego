package debugger

import "testing"

func Test_identationCounts(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		line   string
		opened int
		closed int
	}{
		{
			name:   "balanced text",
			line:   "if (x) { y }",
			opened: 2,
			closed: 2,
		},
		{
			name:   "unbalanced opens",
			line:   "if (x) { y",
			opened: 2,
			closed: 1,
		},
		{
			name:   "unbalanced closes",
			line:   "if (x) { y } }",
			opened: 2,
			closed: 3,
		},
		{
			name:   "unbalanced text with string literal",
			line:   `if (x) { y + "{"`,
			opened: 2,
			closed: 1,
		},
		{
			name:   "balanced text with string literal",
			line:   `if (x) { y + "{"}`,
			opened: 2,
			closed: 2,
		},

		{
			name:   "empty text",
			line:   "",
			opened: 0,
			closed: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opened, closed := identationCounts(tt.line)

			if opened != tt.opened {
				t.Errorf("identationCounts() opened = %v, want %v", opened, tt.opened)
			}

			if closed != tt.closed {
				t.Errorf("identationCounts() closed = %v, want %v", closed, tt.closed)
			}
		})
	}
}
