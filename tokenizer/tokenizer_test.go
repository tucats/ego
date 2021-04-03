package tokenizer

import (
	"reflect"
	"testing"
)

func TestTokenize(t *testing.T) {
	type args struct {
		src string
	}

	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "compound token",
			args: args{
				src: "{}",
			},
			want: []string{"{}"},
		},
		{
			name: "embedded compound token",
			args: args{
				src: "stuff{}here",
			},
			want: []string{"stuff", "{}", "here"},
		},
		{
			name: "interface{} compound token",
			args: args{
				src: "var x interface{}",
			},
			want: []string{"var", "x", "interface{}"},
		},
		{
			name: "elipsis compound token",
			args: args{
				src: "fmt(stuff...)",
			},
			want: []string{"fmt", "(", "stuff", "...", ")"},
		},
		{
			name: "assignment, LEQ compound tokens",
			args: args{
				src: "a := 5 <= 6",
			},
			want: []string{"a", ":=", "5", "<=", "6"},
		},
		{
			name: "channel compound tokens",
			args: args{
				src: "x <- 55 -> stuff",
			},
			want: []string{"x", "<-", "55", "->", "stuff"},
		},
		{
			name: "Simple alphanumeric name",
			args: args{
				src: "wage55",
			},
			want: []string{"wage55"},
		},
		{
			name: "Integer expression with spaces",
			args: args{
				src: "11 + 15",
			},
			want: []string{"11", "+", "15"},
		},
		{
			name: "Integer expression without spaces",
			args: args{
				src: "11+15",
			},
			want: []string{"11", "+", "15"},
		},
		{
			name: "String expression with spaces",
			args: args{
				src: "name + \"User\"",
			},
			want: []string{"name", "+", "\"User\""},
		},
		{
			name: "Float expression",
			args: args{
				src: "3.14 + 2",
			},
			want: []string{"3.14", "+", "2"},
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tk := New(tt.args.src)
			got := tk.Tokens
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Tokenize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSymbol(t *testing.T) {
	type args struct {
		s string
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "alphabetic",
			args: args{"foobar"},
			want: true,
		},
		{
			name: "alphanumeric",
			args: args{"foobar55"},
			want: true,
		},
		{
			name: "underscore",
			args: args{"_"},
			want: true,
		},
		{
			name: "has underscore",
			args: args{"cat_house"},
			want: true,
		},
		{
			name: "digit first",
			args: args{"5foobar"},
			want: false,
		},
		{
			name: "special char",
			args: args{"!foobar"},
			want: false,
		},

		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSymbol(tt.args.s); got != tt.want {
				t.Errorf("IsSymbol() = %v, want %v", got, tt.want)
			}
		})
	}
}
