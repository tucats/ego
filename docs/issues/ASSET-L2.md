# ASSET-L2 — Path normalization uses fragile string removal instead of `filepath.Clean`

**Affected file:** `server/assets/handler.go:359` — `normalizeAssetPath()`

**Description:**  
The function removed `..` components by calling `strings.ReplaceAll(path, "..", "")`
twice — once on the relative path and once on the fully-joined path. While no
active bypass was demonstrated against this specific code, naive string removal
is a well-known anti-pattern for path traversal defense: variations such as
`....//` survive the replacement and produce unexpected results after
`filepath.Join` normalizes double-slashes. The `AssetsHandler` check at line 69
only guards against `/../` mid-path and would not catch a traversal that reached
`normalizeAssetPath` directly (e.g. via the exported `Loader` function called
from dashboard handlers).

**Recommendation:**  
Use `filepath.Clean(filepath.Join(root, path))` for canonical resolution, then
verify the result is still inside `root` with a `strings.HasPrefix` confinement
check. Return a guaranteed-nonexistent path on confinement failure so the
caller handles it uniformly as a 404.

**Resolution (April 2026):**  
`normalizeAssetPath` rewritten to: (1) compute `root` as before, (2) call
`filepath.Clean(filepath.Join(root, path))` for one-pass canonical resolution,
(3) verify the result starts with `root + string(filepath.Separator)`, and
(4) return `filepath.Join(root, "__invalid__")` on confinement failure.
The loop that stripped leading dots/slashes and both `strings.ReplaceAll`
calls have been removed.

