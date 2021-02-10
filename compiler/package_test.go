package compiler

import (
	"testing"

	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/errors"
)

func TestCompiler_ReadDirectory(t *testing.T) {
	tests := []struct {
		name      string
		args      string
		wantEmpty bool
		wantErr   bool
	}{
		{
			name:      "read a directory that exists",
			args:      "strings",
			wantEmpty: false,
			wantErr:   false,
		},
		{

			name:      "read a directory that does not exist",
			args:      "xyzzy",
			wantEmpty: false,
			wantErr:   true,
		},
		// TODO: Add test cases.
	}

	_ = persistence.Load("ego", "")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Compiler{}
			got, err := c.ReadDirectory(tt.args)
			if (!errors.Nil(err)) != tt.wantErr {
				t.Errorf("Compiler.ReadDirectory() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if tt.wantEmpty && len(got) > 0 {
				t.Errorf("Compiler.ReadDirectory() = %v, want empty string", got)
			}
		})
	}
}
