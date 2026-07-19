package io

import (
	"path/filepath"
	"testing"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
)

func Test_sandboxName(t *testing.T) {
	// When the sandbox root exists on disk, sandboxName now resolves symlinks
	// (CODE-M4) and returns the real path. On macOS "/tmp" is itself a symlink
	// to "/private/tmp", so the expected result for a real, existing sandbox
	// root must be computed rather than assumed to equal the raw string join.
	// On Linux "/tmp" is a plain directory and EvalSymlinks is a no-op.
	tmpResolved, err := filepath.EvalSymlinks("/tmp")
	if err != nil {
		tmpResolved = "/tmp"
	}

	tests := []struct {
		name    string
		sandbox string
		want    string
	}{
		{
			name:    "/tmp/foo/odd../name",
			sandbox: "/bar",
			want:    "/bar/tmp/foo/odd../name",
		},
		{
			// A traversal attempt is clamped back to the sandbox root
			// itself rather than escaping it. The previous implementation
			// pattern-matched raw ".." substrings and replaced them with
			// the literal text "<invalid path>", which was both easy to
			// evade (see Test_sandboxName_BareDoubleDotEscapes below, which
			// it did not catch at all) and a confusing result to hand back
			// to a real filesystem call.
			name:    "../../tmp/foo",
			sandbox: "/bar",
			want:    "/bar",
		},
		{
			name:    "/tmp/foo/../..",
			sandbox: "/bar",
			want:    "/bar",
		},
		{
			name:    "/tmp/foo",
			sandbox: "",
			want:    "/tmp/foo",
		},
		{
			name:    "/tmp/foo",
			sandbox: "/tmp",
			want:    filepath.Join(tmpResolved, "foo"),
		},
		{
			name:    "/tmp/foo",
			sandbox: "/bar",
			want:    "/bar/tmp/foo",
		},
		{
			name:    "tmp/foo",
			sandbox: "/bar",
			want:    "/bar/tmp/foo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings.Set(defs.SandboxPathSetting, tt.sandbox)

			if got := sandboxName(true, tt.name); got != tt.want {
				t.Errorf("sandboxName() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test_sandboxName_BareDoubleDotEscapes covers a case the previous
// implementation's raw-substring pattern match ("../", "/..", "/../") never
// caught at all: a path that is *only* "..", with no surrounding slashes.
// filepath.Join("/bar", "..") happily resolves to "/" (the parent of
// "/bar"), which the old code returned completely unmodified. This must now
// be clamped back to the sandbox root.
func Test_sandboxName_BareDoubleDotEscapes(t *testing.T) {
	settings.Set(defs.SandboxPathSetting, "/bar")

	if got := sandboxName(true, ".."); got != "/bar" {
		t.Errorf("sandboxName(true, \"..\") = %v, want %v (clamped to sandbox root)", got, "/bar")
	}
}
