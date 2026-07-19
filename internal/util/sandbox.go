package util

import (
	"os"
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
//
// String cleaning alone still cannot catch a symlink *inside* the sandbox that
// points to a target outside it (e.g. "sandbox/escape -> /etc", then
// io.Open("escape/passwd")): the raw string "sandbox/escape/passwd" is
// trivially within the root, yet the OS follows the symlink to /etc/passwd on
// open. To close that gap, once a string-safe candidate is chosen it is passed
// through resolveWithinSandbox, which resolves symlinks against the real
// filesystem and re-verifies containment before returning.
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
		return resolveWithinSandbox(cleanedPath, sandboxRoot)
	}

	// Otherwise, join path onto sandboxRoot and verify the cleaned result
	// still lives inside sandboxRoot.
	if joined := filepath.Clean(filepath.Join(sandboxRoot, path)); withinRoot(joined, sandboxRoot) {
		return resolveWithinSandbox(joined, sandboxRoot)
	}

	// path attempted to escape sandboxRoot -- clamp it back to the sandbox
	// root itself rather than returning a location outside it.
	return sandboxRoot
}

// resolveWithinSandbox takes a candidate path already confirmed to be within
// sandboxRoot as a *string* and additionally guarantees it does not escape via
// a symlink on the real filesystem. It resolves symlinks on the longest
// portion of candidate that actually exists on disk and re-checks containment
// against the resolved sandbox root. If the resolved location escapes the
// sandbox, the sandbox root is returned instead.
//
// When the sandbox root does not exist on disk, there is nothing inside it to
// follow, so the string candidate is returned unchanged -- this preserves the
// pure-string behavior relied on by callers (and tests) that operate on
// not-yet-created sandbox directories.
func resolveWithinSandbox(candidate, sandboxRoot string) string {
	// Resolve the sandbox root itself; if it is a symlink, its real location
	// is the true boundary to measure containment against. If it does not
	// exist, fall back to the string candidate.
	resolvedRoot, err := filepath.EvalSymlinks(sandboxRoot)
	if err != nil {
		return candidate
	}

	// Walk up from candidate to the longest ancestor that exists on disk.
	// Anything below that does not yet exist and therefore cannot be a
	// symlink, so only the existing prefix needs symlink resolution. Because
	// the sandbox root exists (EvalSymlinks succeeded above) and candidate is
	// a string-descendant of it, this walk stops at sandboxRoot at the latest.
	existing := candidate

	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		}

		parent := filepath.Dir(existing)
		if parent == existing {
			// Reached the filesystem root without finding an existing path;
			// nothing to resolve.
			return candidate
		}

		existing = parent
	}

	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return candidate
	}

	// A symlink in the existing prefix redirected outside the sandbox -- clamp
	// back to the sandbox root rather than handing back an outside location.
	if !withinRoot(resolved, resolvedRoot) {
		return sandboxRoot
	}

	// Re-attach the not-yet-existing remainder of candidate onto the resolved
	// (symlink-free) prefix so callers creating new files still get the full
	// intended path.
	if rel, err := filepath.Rel(existing, candidate); err == nil && rel != "." {
		return filepath.Join(resolved, rel)
	}

	return resolved
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
