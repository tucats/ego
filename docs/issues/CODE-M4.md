# CODE-M4 — Sandbox I/O path confinement can be bypassed via symlinks

**Affected file:** `runtime/io/io.go:121` — `sandboxName()`

```go
func sandboxName(flag bool, path string) string {
    if sandboxPrefix := settings.Get(defs.SandboxPathSetting); flag && sandboxPrefix != "" {
        if strings.HasPrefix(path, "../") || ... {
            path = strings.ReplaceAll(path, "..", "<invalid path>")
        }
        if strings.HasPrefix(path, sandboxPrefix) {
            return path
        }
        return filepath.Join(sandboxPrefix, path)
    }
    return path
}
```

**Description:**  
`sandboxName` prevents `..`-based directory traversal in the path string itself,
but does not resolve symlinks before checking or returning the path. If user
code (or a prior operation) creates a symlink inside the sandbox directory that
points to a location outside it, subsequent `io.Open` and `io.ReadDir` calls
will follow that symlink and access the target path, bypassing the confinement
entirely. For example:

```text
sandbox/escape -> /etc
io.Open("escape/passwd")  // sandboxName returns sandbox/escape/passwd
                           // os.OpenFile follows symlink → /etc/passwd
```

This is a well-known weakness of path-prefix confinement without symlink
resolution: the check is done on the path string rather than on the filesystem
object it ultimately refers to.

**Recommendation:**  
After computing the candidate path, resolve all symlinks with
`filepath.EvalSymlinks` and verify the result still falls under the sandbox
root before opening or listing:

```go
resolved, err := filepath.EvalSymlinks(candidate)
if err != nil || !strings.HasPrefix(resolved, sandboxPrefix+string(filepath.Separator)) {
    return filepath.Join(sandboxPrefix, "__invalid__")
}
return resolved
```

Note that `filepath.EvalSymlinks` requires the file to already exist; for
write operations (creating new files) that will not yet exist, verify the
parent directory instead.

**Status:** Resolved. The three per-package `sandboxName` implementations were
previously consolidated into `util.SandboxJoin`
(`internal/util/sandbox.go`), which every I/O caller now routes through. That
function now performs symlink resolution in addition to string containment: a
string-safe candidate is passed to `resolveWithinSandbox`, which resolves the
longest actually-existing prefix of the candidate with `filepath.EvalSymlinks`
and re-verifies containment against the resolved sandbox root. If the resolved
location escapes the sandbox it is clamped back to the sandbox root; the
not-yet-existing remainder (for write/create operations) is re-attached onto
the resolved, symlink-free prefix. When the sandbox root does not exist on
disk there is nothing to follow, so the pure-string result is returned
unchanged. Covered by `TestSandboxJoin_SymlinkEscape` and
`TestSandboxJoin_SymlinkInsideAllowed` in `internal/util/sandbox_test.go`.

