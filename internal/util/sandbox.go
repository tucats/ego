package util

import (
	"path/filepath"
	"strings"
)

// SandboxJoin resolves path against sandboxRoot and guarantees the result can
// never reference a location outside sandboxRoot, regardless of how path is
// spelled -- ".." segments anywhere in the string, a bare "..", an absolute
// path pointing elsewhere, or a sibling directory that merely shares
// sandboxRoot as a string prefix (e.g. "/sandbox-evil" against a sandbox
// root of "/sandbox"). If sandboxRoot is empty, path is returned unchanged.
//
// This consolidates three previously-independent implementations (in
// bytecode.callNative's sandboxName, runtime/io's sandboxName, and
// runtime/os's sandboxName) that joined the two paths with filepath.Join and
// trusted the result, or at best pattern-matched a few raw substrings like
// "../" before joining -- both approaches a caller could evade (a bare ".."
// argument was never caught by the substring check; filepath.Join silently
// resolves ".." segments above sandboxRoot with no verification at all).
// This version instead joins, cleans, and verifies containment against the
// normalized result, which cannot be fooled by how the raw input is spelled.
func SandboxJoin(sandboxRoot, path string) string {
	if sandboxRoot == "" {
		return path
	}

	sandboxRoot = filepath.Clean(sandboxRoot)

	// If path is already an absolute path safely inside sandboxRoot (for
	// example, a caller re-using a path a previous sandboxed call already
	// returned), leave it as-is rather than joining it onto sandboxRoot a
	// second time.
	if cleanedPath := filepath.Clean(path); withinRoot(cleanedPath, sandboxRoot) {
		return cleanedPath
	}

	// Otherwise, join path onto sandboxRoot and verify the cleaned result
	// still lives inside sandboxRoot.
	if joined := filepath.Clean(filepath.Join(sandboxRoot, path)); withinRoot(joined, sandboxRoot) {
		return joined
	}

	// path attempted to escape sandboxRoot -- clamp it back to the sandbox
	// root itself rather than returning a location outside it.
	return sandboxRoot
}

// withinRoot reports whether cleanPath (already filepath.Clean'd) is equal to
// root or is a descendant of it. Both arguments must already be cleaned.
//
// This is expressed with filepath.Rel rather than a bare strings.HasPrefix
// for two reasons: a naive prefix check would treat a sibling directory that
// merely shares root as a string prefix (e.g. "/sandbox-evil" against a root
// of "/sandbox") as if it were contained within root; and when root is "."
// (a relative sandbox root, as used by tests), filepath.Join/Clean silently
// strips the leading "./" from any joined result, which breaks a
// root-plus-separator prefix check entirely even for a clearly-contained
// path. filepath.Rel sidesteps both problems by computing the relative path
// from root to cleanPath and checking whether that climbs above root (a
// leading ".." segment) instead of pattern-matching the raw strings.
func withinRoot(cleanPath, root string) bool {
	if cleanPath == root {
		return true
	}

	rel, err := filepath.Rel(root, cleanPath)
	if err != nil {
		return false
	}

	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
