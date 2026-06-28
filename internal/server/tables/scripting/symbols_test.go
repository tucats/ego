package scripting

import "testing"

func Test_applySymbolsToString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
		fails bool
	}{
		{
			name:  "no subs at all",
			input: "answer is 101",
			want:  "answer is 101",
			fails: false,
		},
		{
			name:  "simple success",
			input: "answer is {{foo}}",
			want:  "answer is 42",
			fails: false,
		},
		{
			name:  "multiple subs",
			input: "answer is {{foo}} name is {{bar}}",
			want:  "answer is 42 name is Tom",
			fails: false,
		},
		{
			name:  "bad sub ",
			input: "answer is {{zorp}}",
			want:  "",
			fails: true,
		},
		{
			name:  "one good and one bad sub ",
			input: "answer is {{foo}} orca is {{zorp}}",
			want:  "",
			fails: true,
		},
	}

	syms := symbolTable{symbols: map[string]any{
		"foo": 42,
		"bar": "Tom",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := applySymbolsToString(100, tt.input, &syms, "unit test")
			if got != tt.want {
				t.Errorf("applySymbolsToString() got = %v, want %v", got, tt.want)
			}

			if (got1 == nil) == tt.fails {
				t.Errorf("applySymbolsToString() unexpected error %v, expected error = %v", got1, tt.fails)
			}
		})
	}
}
