package cipher

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func Test_hash(t *testing.T) {
	tests := []struct {
		name    string
		args    data.List
		want    interface{}
		wantErr bool
	}{
		{
			name:    "missing argument",
			args:    data.NewList(),
			wantErr: true,
		},
		{
			name:    "too many arguments",
			args:    data.NewList("test", "foo"),
			wantErr: true,
		},
		{
			name: "empty string",
			args: data.NewList(""),
			want: "d41d8cd98f00b204e9800998ecf8427e",
		},
		{
			name: "simple string",
			args: data.NewList("foo"),
			want: "acbd18db4cc2f85cedef654fccc4a4d8",
		},
		{
			name: "unicode string",
			args: data.NewList("\u2318foo\u2318"),
			want: "584aeef41c6a96bfad9dcc3bc217dd9a",
		},
		{
			name: "long string",
			args: data.NewList(
				"Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum."),
			want: "db89bb5ceab87f9c0fcc2ab36c189c2c",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := symbols.NewRootSymbolTable("testing")

			got, err := hash(s, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("hash() error = %v, wantErr %v", err, tt.wantErr)
				
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("hash() = %v, want %v", got, tt.want)
			}
		})
	}
}
