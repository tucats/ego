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
		want []Token
	}{
		{
			name: "Float expression",
			args: args{
				src: "3.14 + 2",
			},
			want: []Token{
				NewFloatToken("3.14"),
				AddToken,
				NewIntegerToken("2"),
				NewSpecialToken(";"),
			},
		},
		{
			name: "compound token",
			args: args{
				src: "{}",
			},
			want: []Token{
				EmptyInitializerToken,
				NewSpecialToken(";"),
			},
		},
		{
			name: "embedded compound token",
			args: args{
				src: "stuff{}here",
			},
			want: []Token{
				NewIdentifierToken("stuff"),
				EmptyInitializerToken,
				NewIdentifierToken("here"),
				NewSpecialToken(";"),
			},
		},
		{
			name: "interface{} compound token",
			args: args{
				src: "var x interface{}",
			},
			want: []Token{
				VarToken,
				NewIdentifierToken("x"),
				EmptyInterfaceToken,
				NewSpecialToken(";"),
			},
		},
		{
			name: "elipsis compound token",
			args: args{
				src: "fmt(stuff...)",
			},
			want: []Token{
				NewIdentifierToken("fmt"),
				StartOfListToken,
				NewIdentifierToken("stuff"),
				VariadicToken,
				EndOfListToken,
				NewSpecialToken(";"),
			},
		},
		{
			name: "assignment, LEQ compound tokens",
			args: args{
				src: "a := 5 <= 6",
			},
			want: []Token{
				NewIdentifierToken("a"),
				DefineToken,
				NewIntegerToken("5"),
				LessThanOrEqualsToken,
				NewIntegerToken("6"),
				NewSpecialToken(";"),
			},
		},
		{
			name: "channel compound tokens",
			args: args{
				src: "x <- 55",
			},
			want: []Token{
				NewIdentifierToken("x"),
				ChannelReceiveToken,
				NewIntegerToken("55"),
				NewSpecialToken(";"),
			},
		},
		{
			name: "Simple alphanumeric name",
			args: args{
				src: "wage55",
			},
			want: []Token{
				NewIdentifierToken("wage55"),
				NewSpecialToken(";"),
			},
		},
		{
			name: "Integer expression with spaces",
			args: args{
				src: "11 + 15",
			},
			want: []Token{
				NewIntegerToken("11"),
				AddToken,
				NewIntegerToken("15"),
				NewSpecialToken(";"),
			},
		},
		{
			name: "Integer expression without spaces",
			args: args{
				src: "11+15",
			},
			want: []Token{
				NewIntegerToken("11"),
				AddToken,
				NewIntegerToken("15"),
				NewSpecialToken(";"),
			},
		},
		{
			name: "String expression with spaces",
			args: args{
				src: "name + \"User\"",
			},
			want: []Token{
				NewIdentifierToken("name"),
				AddToken,
				NewStringToken("User"),
				NewSpecialToken(";"),
			},
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tk := New(tt.args.src, true)
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

func TestTokenizer_Remainder(t *testing.T) {
	tests := []struct {
		name  string
		count int
		want  string
	}{
		{
			name:  "value=1/2/3",
			count: 2,
			want:  "1/2/3",
		},
		{
			name:  "value= 1/2/3",
			count: 1,
			want:  "= 1/2/3",
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := New(tt.name, false)
			tr.Set(tt.count)

			if got := tr.Remainder(); got != tt.want {
				t.Errorf("Tokenizer.Remainder() = %v, want %v", got, tt.want)
			}
		})
	}
}
