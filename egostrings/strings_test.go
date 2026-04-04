package egostrings

import (
	"testing"

	"github.com/google/uuid"
)

func TestJoin(t *testing.T) {
	tests := []struct {
		name      string
		separator string
		values    []any
		want      string
	}{
		// Scalar values
		{
			name:      "single string scalar",
			separator: ",",
			values:    []any{"hello"},
			want:      "hello",
		},
		{
			name:      "two string scalars",
			separator: ", ",
			values:    []any{"hello", "world"},
			want:      "hello, world",
		},
		{
			name:      "integer scalar",
			separator: "-",
			values:    []any{42},
			want:      "42",
		},
		{
			name:      "bool scalar",
			separator: ",",
			values:    []any{true},
			want:      "true",
		},
		{
			name:      "float scalar",
			separator: ",",
			values:    []any{3.14},
			want:      "3.14",
		},
		{
			name:      "mixed scalars",
			separator: "|",
			values:    []any{"x", 1, true},
			want:      "x|1|true",
		},

		// []string slice
		{
			name:      "string slice",
			separator: ", ",
			values:    []any{[]string{"a", "b", "c"}},
			want:      "a, b, c",
		},
		{
			name:      "string slice with colon separator",
			separator: ":",
			values:    []any{[]string{"foo", "bar"}},
			want:      "foo:bar",
		},

		// []any slice
		{
			name:      "any slice",
			separator: "-",
			values:    []any{[]any{"x", 2, false}},
			want:      "x-2-false",
		},

		// []int slice
		{
			name:      "int slice",
			separator: "+",
			values:    []any{[]int{1, 2, 3}},
			want:      "1+2+3",
		},

		// []float64 slice
		{
			name:      "float64 slice",
			separator: " ",
			values:    []any{[]float64{1.5, 2.5}},
			want:      "1.500000 2.500000",
		},

		// []bool slice
		{
			name:      "bool slice",
			separator: "/",
			values:    []any{[]bool{true, false, true}},
			want:      "true/false/true",
		},

		// []rune slice
		{
			name:      "rune slice",
			separator: "",
			values:    []any{[]rune{'H', 'i', '!'}},
			want:      "Hi!",
		},

		// Mixed slices and scalars in one call
		{
			name:      "slice then scalar",
			separator: ",",
			values:    []any{[]string{"a", "b"}, "c"},
			want:      "a,b,c",
		},
		{
			name:      "scalar then slice",
			separator: ",",
			values:    []any{"z", []string{"x", "y"}},
			want:      "z,x,y",
		},

		// Edge cases
		{
			name:      "empty separator",
			separator: "",
			values:    []any{"foo", "bar"},
			want:      "foobar",
		},
		{
			name:      "no values",
			separator: ",",
			values:    []any{},
			want:      "",
		},
		{
			name:      "empty string slice",
			separator: ",",
			values:    []any{[]string{}},
			want:      "",
		},
		{
			name:      "single element string slice",
			separator: ",",
			values:    []any{[]string{"only"}},
			want:      "only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Join(tt.separator, tt.values...)
			if got != tt.want {
				t.Errorf("Join(%q, %v) = %q, want %q", tt.separator, tt.values, got, tt.want)
			}
		})
	}
}

func TestGibberish(t *testing.T) {
	// Set up test cases
	tests := []struct {
		name string
		u    uuid.UUID
		want string
	}{
		{
			name: "test 1 synthetic UUID",
			u:    uuid.MustParse("00000000-0000-0000-0000-000000000000"),
			want: "-nil-",
		},
		{
			name: "test 2 synthetic UUID",
			u:    uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			want: "b",
		},
		{
			name: "test 3 synthetic UUID",
			u:    uuid.MustParse("10000000-0000-0000-0000-000000000000"),
			want: "aaaaaaaaaaaab",
		},
		{
			name: "test 1 random UUID",
			u:    uuid.MustParse("ab34d542-a437-408a-b0ca-38ea5d78696f"),
			want: "rm4szqj72tubkesqdukixgpyk",
		},
		{
			name: "test 2 random UUID",
			u:    uuid.MustParse("4867dd02-3b98-4d68-9843-06179aa8553e"),
			want: "8jxskp8cg2simvs37ia783se",
		},
	}

	// Run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Gibberish(tt.u)
			if got != tt.want {
				t.Errorf("Gibberish(%s) = %v, want %v", tt.u, got, tt.want)
			}
		})
	}
}

func TestAtoi(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		// Decimal integers
		{name: "zero", input: "0", want: 0},
		{name: "positive decimal", input: "42", want: 42},
		{name: "negative decimal", input: "-7", want: -7},
		{name: "leading/trailing spaces", input: "  15  ", want: 15},

		// Hexadecimal
		{name: "hex lowercase", input: "0xff", want: 255},
		{name: "hex uppercase prefix", input: "0XFF", want: 255},
		{name: "hex single digit", input: "0x1", want: 1},
		{name: "hex zero", input: "0x0", want: 0},

		// Octal
		{name: "octal", input: "0o17", want: 15},
		{name: "octal zero", input: "0o0", want: 0},

		// Binary
		{name: "binary", input: "0b1010", want: 10},
		{name: "binary one", input: "0b1", want: 1},
		{name: "binary zero", input: "0b0", want: 0},

		// Rune literals
		{name: "rune A", input: "'A'", want: 65},
		{name: "rune space", input: "' '", want: 32},
		{name: "rune zero digit", input: "'0'", want: 48},

		// Error cases
		{name: "empty string", input: "", want: 0, wantErr: true},
		{name: "letters only", input: "abc", want: 0, wantErr: true},
		{name: "invalid hex", input: "0xGG", want: 0, wantErr: true},
		{name: "invalid binary", input: "0b2", want: 0, wantErr: true},
		{name: "multi-char rune", input: "'AB'", want: 0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Atoi(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Atoi(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Atoi(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestEscape(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "string with no special characters",
			input: "Hello, World!",
			want:  "Hello, World!",
		},
		{
			name:  "string with a double quote",
			input: `Hello, "World!"`,
			want:  `Hello, \"World!\"`,
		},
		{
			name: "string with a newline",
			input: `Hello,
World!`,
			want: `Hello,\nWorld!`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Escape(tt.input)
			if got != tt.want {
				t.Errorf("Escape(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
