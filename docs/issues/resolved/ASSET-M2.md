# ASSET-M2 — Error response discloses server filesystem paths

**Affected file:** `server/assets/handler.go:131` — `AssetsHandler()`

**Description:**  
When `Loader` returned an error (file not found, permission denied, etc.), the
handler embedded the raw OS error string — which contains the absolute
filesystem path — directly in the JSON response body:
`{"err": "open /home/tom/ego/lib/foo.txt: no such file or directory"}`. A
partial `strings.ReplaceAll` only stripped the `services` subdirectory; all
other paths were exposed verbatim. An unauthenticated caller could use this to
map the server's directory layout, confirm the existence of files, and discover
the configured `EGO_PATH`.

**Recommendation:**  
Return a fixed generic message to the caller and keep full error detail in the
server log only.

**Resolution (April 2026):**  
The error branch in `AssetsHandler` now writes the literal string
`{"err": "asset not found"}` to the response for all load failures. The
original error (including the real path) continues to be written to the
`AssetLogger` via the existing `asset.load.error` log key.

