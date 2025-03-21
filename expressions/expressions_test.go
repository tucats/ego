// Package expressions is a simple expression evaluator. It supports
// a rudimentary symbol table with scoping, and knows about four data
// types (string, integer, double, and boolean). It does type casting as
// need automatically.
package expressions

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		want    interface{}
		wantErr bool
	}{
		{
			name: "Concatenate array",
			expr: "append([1,2], [3,4]...)",
			want: []interface{}{1, 2, 3, 4},
		},
		{
			name: "index of string function",
			expr: "index(name, \"o\")",
			want: 2,
		},
		{
			name: "Bitwise AND",
			expr: "7 & 1",
			want: 1,
		}, {
			name: "Bitwise OR",
			expr: "8|3",
			want: 11,
		}, {
			name: "Index into array",
			expr: "a[1]",
			want: "tom",
		},
		{
			name: "long quote",
			expr: "`test \"string\" of text`",
			want: "test \"string\" of text",
		},
		{
			name: "Cast bool to string",
			expr: "string(b) + string(!b)",
			want: "truefalse",
		},
		{
			name: "Simple unary not",
			expr: "(!b)",
			want: false,
		},
		{
			name: "Simple division",
			expr: "i / 7",
			want: 6,
		},
		{
			name: "String subtraction",
			expr: "\"test\" - \"st\"",
			want: "te",
		},
		{
			name: "Alphanumeric  symbol names",
			expr: "roman12 + \".\"",
			want: "XII.",
		},
		{
			name: "Simple addition",
			expr: "5 + i",
			want: 47,
		},
		{
			name: "Simple integer multiplication",
			expr: "5 * 20",
			want: 100,
		},
		{
			name: "Simple floating multiplication",
			expr: "5. * 20.",
			want: 100.,
		},
		{
			name:    "Invalid type for  multiplication",
			expr:    "true * \"many\"",
			want:    nil,
			wantErr: true,
		},
		{
			name: "Simple subtraction",
			expr: "pi - 1",
			want: 2.14,
		},
		{
			name: "Float division",
			expr: "10.0 / 4.0",
			want: 2.5,
		},
		{
			name: "Invalid division",
			expr: "\"house\" / \"cat\" ",
			want: nil,
		},
		{
			name: "Integer divide by zero ",
			expr: "17 / 0",
			want: nil,
		},
		{
			name: "Float divide by zero ",
			expr: "3.33 / 0.0 ",
			want: nil,
		},
		{
			name: "Order precedence",
			expr: "5 + i / 7",
			want: 11,
		},
		{
			name: "Order precedence with parens",
			expr: "(5 + i) * 2",
			want: 94,
		},
		{
			name: "Multiple paren terms",
			expr: "(i==42) && (name==\"Tom\")",
			want: true,
		},
		{
			name: "Invalid multiple paren terms",
			expr: "(i==42) && (name==\"Tom\"",
			want: nil,
		},
		{
			name: "Unary negation of single term",
			expr: "-i",
			want: -42,
		},
		{
			name: "Invalid unary negation",
			expr: "-name",
			want: "moT",
		},
		{
			name: "Unary negation of diadic operator",
			expr: "43 + -i",
			want: 1,
		},
		{
			name: "Unary negation of subexpression",
			expr: "-(5+pi)",
			want: -8.14,
		},
		{
			name: "Type promotion bool to int",
			expr: "b + 3",
			want: 4,
		},
		{
			name: "Type promotion int to string",
			expr: "5 + name",
			want: "5Tom",
		},
		{
			name: "Type promotion int to float64",
			expr: "pi + 5",
			want: 8.14,
		},
		{
			name: "Type coercion bool to int",
			expr: "int(true) + int(false)",
			want: 1,
		},
		{
			name: "Type coercion int to bool",
			expr: "i && true",
			want: true,
		},
		{
			name: "Type coercion bool to string",
			expr: "\"true\" || false",
			want: defs.True,
		},
		{
			name: "Type coercion string to bool",
			expr: "bool(\"true\") && true",
			want: true,
		},
		{
			name:    "Invalid type coercion string to bool",
			expr:    "\"bob\" || false",
			want:    nil,
			wantErr: true,
		},
		{
			name: "Cast value to int",
			expr: "int(3.14)",
			want: 3,
		},
		{
			name: "Cast value to float64",
			expr: "float64(55)",
			want: 55.,
		},
		{
			name: "Cast bool to float64",
			expr: "float64(b)",
			want: 1.,
		},
		{
			name: "Cast value to bool",
			expr: "bool(5)",
			want: true,
		},
		{
			name: "Cast float64 to string",
			expr: "string(003.14)",
			want: "3.14",
		},
		{
			name: "Cast int to string",
			expr: "string(i)",
			want: "42",
		},

		{
			name: "Homogeneous array constant",
			expr: "[1,2]",
			want: []interface{}{1, 2},
		},
		{
			name: "Heterogeneous array constant",
			expr: "[true,name, 33.5]",
			want: []interface{}{true, "Tom", 33.5},
		},
		{
			name: "Invalid argument list to function",
			expr: "len(1 3)",
			want: nil,
		},
		{
			name: "Incomplete argument list to function",
			expr: "len(13",
			want: nil,
		},
		{
			name: "map constant member",
			expr: "{name:\"Tom\", age:50}.age",
			want: 50,
		},
		{
			name: "map constant nested array member",
			expr: "{name:\"Tom\",sizes:[10, 12], age:50}.sizes",
			want: []interface{}{10, 12},
		},
		{
			name: "map constant nested array member indexed",
			expr: "{name:\"Tom\",sizes:[10, 12], age:50}.sizes[1]",
			want: 12,
		},
		{
			name: "len of string function",
			expr: "len(name) + 4",
			want: 7,
		},
		{
			name: "len of array function",
			expr: "len(a) + 4",
			want: 8,
		},
		{
			name: "index not found function",
			expr: "index(name, \"g\")",
			want: 0,
		},
		{
			name: "index of array function",
			expr: "index(a, false)",
			want: 3,
		},
		{
			name: "index of array not found",
			expr: "index(a, 55.5)",
			want: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a common symbol table.
			s := symbols.NewSymbolTable(tt.name)
			s.SetAlways("i", 42)
			s.SetAlways("pi", 3.14)
			s.SetAlways("name", "Tom")
			s.SetAlways("b", true)
			s.SetAlways("roman12", "XII")
			s.SetAlways("a", data.NewArrayFromList(data.InterfaceType, data.NewList(1, "tom", 33., false)))
			s.Root().SetAlways(defs.ExtensionsVariable, true)

			// Compile the string and evaluate using the symbol table
			v1, err := Evaluate(tt.expr, s)
			if errors.Equals(err, errors.ErrStop) {
				err = nil
			}

			if err != nil && tt.want != nil {
				t.Errorf("Expression test, unexpected error %v", err)
			} else {
				if array, ok := v1.(*data.Array); ok {
					if !reflect.DeepEqual(array.BaseArray(), tt.want) {
						t.Errorf("Expression test, got %v, want %v", v1, tt.want)
					}
				} else if !reflect.DeepEqual(v1, tt.want) {
					t.Errorf("Expression test, \n  got %v\n want %v", v1, tt.want)
				}
			}
		})
	}
}
