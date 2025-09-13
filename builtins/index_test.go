package builtins

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
)

func TestFunctionIndex(t *testing.T) {
	tests := []struct {
		name    string
		args    data.List
		want    any
		wantErr bool
	}{
		{
			name: "index found",
			args: data.NewList("string of text", "of"),
			want: 8,
		},
		{
			name: "index not found",
			args: data.NewList("string of text", "burp"),
			want: 0,
		},
		{
			name: "empty source string",
			args: data.NewList("", "burp"),
			want: 0,
		},
		{
			name: "empty test string",
			args: data.NewList("string of text", ""),
			want: 1,
		},
		{
			name: "non-string test",
			args: data.NewList("A1B2C3D4", 3),
			want: 6,
		},
		{
			name: "array index",
			args: data.NewList(
				data.NewArrayFromInterfaces(data.InterfaceType, "tom", 3.14, true), 3.14),
			want: 1,
		},
		{
			name: "array not found",
			args: data.NewList(
				data.NewArrayFromInterfaces(data.InterfaceType, "tom", 3.14, true), false),
			want: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We will need a symbol table so the Index function can find out
			// if it is allowed or not.
			s := symbols.NewSymbolTable("testing")
			s.Root().SetAlways(defs.ExtensionsVariable, true)

			got, err := Index(s, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("FunctionIndex() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FunctionIndex() = %v, want %v", got, tt.want)
			}
		})
	}
}
