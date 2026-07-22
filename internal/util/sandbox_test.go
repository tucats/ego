package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSandboxJoin(t *testing.T) {
	const root = "/sandbox/root"

	tests := []struct {
		name string
		path string
		want string
	}{
		{"no traversal", "file.txt", "/sandbox/root/file.txt"},
		{"subdirectory", "sub/file.txt", "/sandbox/root/sub/file.txt"},
		{"leading dotslash", "./file.txt", "/sandbox/root/file.txt"},
		{"single traversal escapes to parent", "../secret.txt", root},
		{"double traversal escapes further", "../../etc/passwd", root},
		{"bare double-dot", "..", root},
		{"traversal buried mid-path", "sub/../../secret.txt", root},
		{"absolute path outside root", "/etc/passwd", "/sandbox/root/etc/passwd"},
		{"absolute path already inside root", "/sandbox/root/sub/file.txt", "/sandbox/root/sub/file.txt"},
		{"sibling directory sharing string prefix", "/sandbox/root-evil/secret.txt", "/sandbox/root/sandbox/root-evil/secret.txt"},
		{"empty path resolves to root", "", root},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SandboxJoin(root, tt.path)
			if got != tt.want {
				t.Errorf("SandboxJoin(%q, %q) = %q, want %q", root, tt.path, got, tt.want)
			}

			if !withinRoot(got, root) {
				t.Errorf("SandboxJoin(%q, %q) = %q escapes sandbox root %q", root, tt.path, got, root)
			}
		})
	}
}

// TestSandboxJoin_SymlinkEscape covers CODE-M4: a symlink created inside the
// sandbox pointing to a location outside it must not let subsequent path
// resolution escape the sandbox. String-only containment cannot catch this;
// SandboxJoin resolves symlinks against the real filesystem and clamps back to
// the sandbox root when the target escapes.
func TestSandboxJoin_SymlinkEscape(t *testing.T) {
	// The sandbox lives under a real temp directory; "outside" is a sibling
	// directory NOT under the sandbox, holding a file we must not be able to
	// reach through the sandbox.
	base := t.TempDir()
	sandbox := filepath.Join(base, "sandbox")
	outside := filepath.Join(base, "outside")

	for _, dir := range []string{sandbox, outside} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %q: %v", dir, err)
		}
	}

	secret := filepath.Join(outside, "passwd")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatalf("write %q: %v", secret, err)
	}

	// escape -> outside, created inside the sandbox.
	if err := os.Symlink(outside, filepath.Join(sandbox, "escape")); err != nil {
		t.Skipf("symlinks unsupported on this platform: %v", err)
	}

	tests := []struct {
		name string
		path string
	}{
		{"symlink to existing outside file", "escape/passwd"},
		{"symlink to not-yet-existing outside file", "escape/newfile.txt"},
		{"bare symlink directory", "escape"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SandboxJoin(sandbox, tt.path)

			// The returned path must not resolve to a location outside the
			// sandbox once its own symlinks are followed.
			resolved, err := filepath.EvalSymlinks(got)
			if err != nil {
				resolved = got
			}

			resolvedSandbox, err := filepath.EvalSymlinks(sandbox)
			if err != nil {
				resolvedSandbox = sandbox
			}

			if !withinRoot(filepath.Clean(resolved), filepath.Clean(resolvedSandbox)) {
				t.Errorf("SandboxJoin(%q, %q) = %q which resolves to %q, outside sandbox %q",
					sandbox, tt.path, got, resolved, resolvedSandbox)
			}
		})
	}
}

// TestSandboxJoin_SymlinkInsideAllowed confirms a symlink that stays inside the
// sandbox is still resolved and honored rather than being rejected.
func TestSandboxJoin_SymlinkInsideAllowed(t *testing.T) {
	sandbox := t.TempDir()

	realPath := filepath.Join(sandbox, "real")
	if err := os.MkdirAll(realPath, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", realPath, err)
	}

	if err := os.WriteFile(filepath.Join(realPath, "file.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if err := os.Symlink(realPath, filepath.Join(sandbox, "link")); err != nil {
		t.Skipf("symlinks unsupported on this platform: %v", err)
	}

	got := SandboxJoin(sandbox, "link/file.txt")

	resolved, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", got, err)
	}

	want, _ := filepath.EvalSymlinks(filepath.Join(realPath, "file.txt"))
	if resolved != want {
		t.Errorf("SandboxJoin(%q, %q) resolved to %q, want %q", sandbox, "link/file.txt", resolved, want)
	}
}

func TestSandboxJoin_NoRootConfigured(t *testing.T) {
	got := SandboxJoin("", "../../etc/passwd")
	if got != "../../etc/passwd" {
		t.Errorf("SandboxJoin(\"\", ...) = %q, want path unchanged when no sandbox root is configured", got)
	}
}

// TestSandboxJoin_RelativeRoot covers root == "." (used by test files via
// the @sandbox directive's path="." clause). filepath.Join/Clean strips the
// leading "./" from any result joined onto a "." root, which previously
// broke a naive prefix-based containment check even for legitimately
// contained paths -- see withinRoot's use of filepath.Rel instead.
func TestSandboxJoin_RelativeRoot(t *testing.T) {
	const root = "."

	tests := []struct {
		name string
		path string
		want string
	}{
		{"same-directory file", "file.txt", "file.txt"},
		{"traversal escapes to parent", "../secret.txt", root},
		{"bare double-dot", "..", root},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SandboxJoin(root, tt.path)
			if got != tt.want {
				t.Errorf("SandboxJoin(%q, %q) = %q, want %q", root, tt.path, got, tt.want)
			}
		})
	}
}
