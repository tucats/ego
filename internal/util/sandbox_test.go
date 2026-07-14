package util

import "testing"

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
