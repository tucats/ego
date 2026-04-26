package fmt

import (
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

// newTestSymbols returns a minimal symbol table suitable for unit tests that
// call the print functions. Most print functions only use the symbol table to
// look up a StdoutWriter; since our tests capture the return value directly
// they do not need a writer to be present.
func newTestSymbols() *symbols.SymbolTable {
	return symbols.NewSymbolTable("test")
}

func Test_stringPrintFormat(t *testing.T) {
	tests := []struct {
		name string
		args data.List
		want string
	}{
		{
			name: "Format string only, no values",
			args: data.NewList("hello"),
			want: "hello",
		},
		{
			name: "Empty arg list returns zero",
			// stringPrintFormat returns int(0) when args is empty, not a string.
			// We test separately below for that case.
			args: data.NewList("%s", "world"),
			want: "world",
		},
		{
			name: "Integer format",
			args: data.NewList("%d", 42),
			want: "42",
		},
		{
			name: "Float format",
			args: data.NewList("%.2f", 3.14159),
			want: "3.14",
		},
		{
			name: "Multiple arguments",
			args: data.NewList("%s is %d years old", "Alice", 30),
			want: "Alice is 30 years old",
		},
		{
			name: "Boolean format",
			args: data.NewList("%v", true),
			want: "true",
		},
		{
			name: "%#v is replaced with %v",
			args: data.NewList("%#v", 99),
			want: "99",
		},
		{
			name: "%T produces Ego type name for int",
			args: data.NewList("%T", 42),
			want: "int",
		},
		{
			name: "%T produces Ego type name for string",
			args: data.NewList("%T", "hello"),
			want: "string",
		},
		{
			name: "%T produces Ego type name for float64",
			args: data.NewList("%T", 3.14),
			want: "float64",
		},
		{
			name: "%T produces Ego type name for bool",
			args: data.NewList("%T", true),
			want: "bool",
		},
		{
			name: "%T mixed with other verbs",
			args: data.NewList("value=%d type=%T", 7, 7),
			want: "value=7 type=int",
		},
		{
			name: "%V produces type-decorated value for int",
			args: data.NewList("%V", 42),
			want: "int(42)",
		},
		{
			name: "%V produces type-decorated value for string",
			args: data.NewList("%V", "hello"),
			want: `"hello"`,
		},
		{
			name: "Padding and alignment",
			args: data.NewList("%10s", "hi"),
			want: "        hi",
		},
		{
			name: "Width and precision on float",
			args: data.NewList("%8.3f", 1.5),
			want: "   1.500",
		},
		{
			// With at least one format argument the string goes through fmt.Sprintf,
			// which converts %% to a literal %. The format-only path (args.Len()==1)
			// returns the raw string and does not perform that conversion.
			name: "%% produces a literal percent when Sprintf is called",
			args: data.NewList("100%% done, n=%d", 3),
			want: "100% done, n=3",
		},
		{
			name: "%% before %T does not shift argument index",
			args: data.NewList("100%% of type %T", 42),
			want: "100% of type int",
		},
		{
			name: "%% before %V does not shift argument index",
			args: data.NewList("value is %% or %V", 7),
			want: "value is % or int(7)",
		},
	}

	s := newTestSymbols()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := stringPrintFormat(s, tt.args)
			if err != nil {
				t.Fatalf("stringPrintFormat() unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("stringPrintFormat() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Test_stringPrintFormat_emptyArgs verifies that an empty argument list returns
// int(0) rather than a string, matching the behaviour documented in the source.
func Test_stringPrintFormat_emptyArgs(t *testing.T) {
	s := newTestSymbols()
	got, err := stringPrintFormat(s, data.NewList())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != 0 {
		t.Errorf("empty args: got %v (%T), want 0 (int)", got, got)
	}
}
