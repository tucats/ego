package compiler

import (
	"testing"

	"github.com/tucats/ego/app-cli/settings"
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

	settings.ProfileDirectory = ".ego"

	err := settings.Load("ego", "default")
	if err != nil {
		t.Error("Unable to initialize settings, ", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Compiler{}

			got, err := c.directoryContents(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Compiler.ReadDirectory() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if tt.wantEmpty && len(got) > 0 {
				t.Errorf("Compiler.ReadDirectory() = %v, want empty string", got)
			}
		})
	}
}
