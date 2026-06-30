package compiler

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

func TestArbitraryCodeFragments(t *testing.T) {
	tests := []struct {
		name string
		text string
		want any
	}{
		// The text of each test contains the entire program snipped to
		// run, which _must_ create a variable named "result" which is compared to
		// the wanted value. The program can be of arbitrary complexity, and need
		// not be a complete program or function.
		{
			name: "decimal integer assignment",
			text: "result := 1",
			want: 1,
		},

		// --- BUG-06 regression tests: ++ and -- on qualified lvalues ----------
		//
		// Each test below creates an array or struct, applies ++ or -- to one
		// of its elements/fields, and stores the modified value in "result" for
		// comparison.  Before the BUG-06 fix these programs produced a compile
		// error "invalid use of auto increment/decrement operation".
		{
			// Array element increment: a[0] starts at 1, after a[0]++ it is 2.
			name: "array element increment (BUG-06)",
			text: `a := []int{1, 2, 3}; a[0]++; result := a[0]`,
			want: 2,
		},
		{
			// Array element decrement: a[1] starts at 2, after a[1]-- it is 1.
			name: "array element decrement (BUG-06)",
			text: `a := []int{1, 2, 3}; a[1]--; result := a[1]`,
			want: 1,
		},
		{
			// Struct field increment using a dynamic (anonymous) struct.
			// s.x starts at 10, after s.x++ it is 11.
			name: "struct field increment (BUG-06)",
			text: `s := {x: 10}; s.x++; result := s.x`,
			want: 11,
		},
		{
			// Struct field decrement: s.y starts at 5, after s.y-- it is 4.
			name: "struct field decrement (BUG-06)",
			text: `s := {y: 5}; s.y--; result := s.y`,
			want: 4,
		},
		{
			// Multiple increments: each ++ must be independent and cumulative.
			// a[0] starts at 0, after three a[0]++ calls it is 3.
			name: "multiple array element increments (BUG-06)",
			text: `a := []int{0, 10, 20}; a[0]++; a[0]++; a[0]++; result := a[0]`,
			want: 3,
		},
		{
			// Other array elements are unaffected by ++ on a[0].
			// a[2] should remain 20 after a[0]++.
			name: "array increment does not affect other elements (BUG-06)",
			text: `a := []int{1, 2, 3}; a[0]++; result := a[2]`,
			want: 3,
		},
		{
			// Array element ++ at a computed (variable) index.
			// a[i] where i=1 starts at 5; after a[i]++ it is 6.
			name: "array element increment with variable index (BUG-06)",
			text: `a := []int{0, 5, 10}; i := 1; a[i]++; result := a[i]`,
			want: 6,
		},
		// --- end BUG-06 regression tests -------------------------------------
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
			if err := RunString(tt.name, s, tt.text); !errors.Nil(err) && err.Error() != errors.ErrStop.Error() {
				t.Errorf("Unexpected error %v", err)
			}

			result, found := s.Get("result")
			if !reflect.DeepEqual(result, tt.want) || !found {
				t.Errorf("Unexpected result; got %v, want %v", result, tt.want)
			}
		})
	}
}
