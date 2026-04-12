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
		want    any
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
			want: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name: "simple string",
			args: data.NewList("foo"),
			want: "2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
		},
		{
			name: "unicode string",
			args: data.NewList("\u2318foo\u2318"),
			want: "569d81944e3314d670db0893bdd471cbb598c25e43a184d9113778c650fdc584",
		},
		{
			name: "long string",
			args: data.NewList(
				"Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum."),
			want: "2d8c2f6d978ca21712b5f6de36c9d31fa8e96a4fa5d8ff8b0188dfb9e7c171bb",
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
