package compiler

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
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
	}

	settings.ProfileDirectory = ".ego"

	err := settings.Load("ego", "default")
	if err != nil {
		t.Error("Unable to initialize settings, ", err)
	}

	// directoryContents() resolves "lib/packages/<name>" relative to the
	// persisted ego.runtime.path setting. Whatever settings.Load() just
	// found on disk (if anything) is a developer machine's own profile, not
	// something this test controls, so it can't be relied on to point at
	// this repo's lib/ directory. Compute the repo root from this test
	// file's own location instead, and set it explicitly: SetDefault writes
	// to the transient override layer that settings.Get() checks first, so
	// it wins regardless of what -- if anything -- Load() found.
	_, thisFile, _, _ := runtime.Caller(0) //nolint:dogsled
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	settings.SetDefault(defs.EgoPathSetting, repoRoot)

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
