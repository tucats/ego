package compiler

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func TestArbitraryCodeFragments(t *testing.T) {
	tests := []struct {
		name string
		text string
		want interface{}
	}{
		// TODO: Add test cases. The text contains the entire program snipped to
		// run, which _must_ create a variable named "result" which is compared to
		// the wanted value. The program can be of arbitrary complexity, and need
		// not be a complete program or function.

		{
			name: "decimal integer assignment",
			text: "result := 1",
			want: 1,
		},
		{
			name: "octal integer assignment",
			text: "result := 0o10",
			want: 8,
		},
		{
			name: "hexadecimal integer assignment",
			text: "result := 0x10",
			want: 16,
		},
		{
			name: "optional error catch",
			text: "result := ?(5/0):-1",
			want: -1,
		},
		{
			name: "Conditional expression",
			text: `result := true?"yes":"no"`,
			want: "yes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := symbols.NewRootSymbolTable(tt.name)
			if err := RunString(tt.name, s, tt.text); err != nil && err.Error() != errors.ErrStop.Error() {
				t.Errorf("Unexpected error %v", err)
			}
			result, found := s.Get("result")
			if !reflect.DeepEqual(result, tt.want) || !found {
				t.Errorf("Unexpected result; got %v, want %v", result, tt.want)
			}
		})
	}
}
