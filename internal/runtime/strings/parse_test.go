package strings

import (
	"testing"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// tokenSpelling extracts the "spelling" field from each struct in the array
// returned by tokenize, for easy comparison against an expected list.
func tokenSpellings(t *testing.T, result any) []string {
	t.Helper()

	arr, ok := result.(*data.Array)
	if !ok {
		t.Fatalf("tokenize() did not return *data.Array, got %T", result)
	}

	spellings := make([]string, arr.Len())

	for i := 0; i < arr.Len(); i++ {
		v, err := arr.Get(i)
		if err != nil {
			t.Fatalf("unexpected error reading array element %d: %v", i, err)
		}

		item, ok := v.(*data.Struct)
		if !ok {
			t.Fatalf("array element %d is not *data.Struct, got %T", i, v)
		}

		spelling, _ := item.Get("spelling")
		spellings[i] = data.String(spelling)
	}

	return spellings
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			// foxed BUG-51: "{}" and "<-" must each be a single compound token,
			// not split into individual characters.
			name: "compound tokens are merged",
			src:  "x{} <- f(3, 4)",
			want: []string{"x", "{}", "<-", "f", "(", "3", ",", "4", ")"},
		},
		{
			name: "explicit semicolon is preserved",
			src:  "a; b",
			want: []string{"a", ";", "b"},
		},
		{
			// Enabling code-mode tokenization (required to merge compound
			// tokens) also enables Go-style automatic semicolon insertion at
			// line ends. Those synthetic semicolons must not leak into the
			// result -- only ones the caller actually typed should appear.
			name: "no synthetic semicolon on single line input",
			src:  "x := 1",
			want: []string{"x", ":=", "1"},
		},
		{
			name: "no synthetic semicolons across multiple lines",
			src:  "x := 1\ny := 2",
			want: []string{"x", ":=", "1", "y", ":=", "2"},
		},
		{
			name: "other compound operators are merged",
			src:  "a := 1; b <= 2 && c",
			want: []string{"a", ":=", "1", ";", "b", "<=", "2", "&&", "c"},
		},
	}

	s := symbols.NewSymbolTable("test")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tokenize(s, data.NewList(tt.src))
			if err != nil {
				t.Fatalf("tokenize() unexpected error: %v", err)
			}

			got := tokenSpellings(t, result)

			if len(got) != len(tt.want) {
				t.Fatalf("tokenize(%q) = %v, want %v", tt.src, got, tt.want)
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("tokenize(%q)[%d] = %q, want %q", tt.src, i, got[i], tt.want[i])
				}
			}
		})
	}
}
