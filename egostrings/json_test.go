package egostrings_test

import (
	"testing"

	"github.com/tucats/ego/egostrings"
)

func TestJSONMinify(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		input string
		want  string
	}{
		{
			name:  "escaped quotes",
			input: `{ "name": "John \"baby face\" Doe" }`,
			want:  `{"name":"John \"baby face\" Doe"}`,
		},
		{
			name:  "empty JSON",
			input: "",
			want:  "",
		},
		{
			name:  "single-line JSON",
			input: `{"name": "John Doe"}`,
			want:  `{"name":"John Doe"}`,
		},
		{
			name: "multi-line JSON",
			input: `
			{
			    "name": "John Doe",
                "age": 30,
                "city": "New York"
			}`,
			want: `{"name":"John Doe","age":30,"city":"New York"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := egostrings.JSONMinify(tt.input)
			if got != tt.want {
				t.Errorf("JSONMinify() = %v, want %v", got, tt.want)
			}
		})
	}
}
