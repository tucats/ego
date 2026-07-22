# ASSET-H1 — DoS via open-ended `Range` header

**Affected file:** `server/assets/handler.go:296` — `readAssetRange()`

**Description:**  
A request carrying `Range: bytes=N-` (open-ended range, no explicit end byte)
leaves the `end` variable at its sentinel value `EndOfData = math.MaxInt64`.
Because `start != StartOfData`, the Loader skips the cache path and calls
`readAssetRange`. Inside that function, `size := end - start` evaluates to
approximately 9.2 EB, and the subsequent `make([]byte, size)` attempts to
allocate that many bytes. The Go runtime raises an out-of-memory condition
before the allocation completes, which — if not recovered — terminates the
server process. A single unauthenticated GET request with a valid asset path
and an open-ended Range header is sufficient to trigger this.

**Recommendation:**  
After calling `os.Stat` to obtain the true file size, clamp `end` to
`totalSize - 1` whenever it equals `EndOfData` or exceeds the file size.
This is the correct RFC 7233 interpretation of an open-ended range.

**Resolution (April 2026):**  
Clamping added in `readAssetRange` immediately after `totalSize = info.Size()`.
`size` is now computed as `end - start + 1` (inclusive, per RFC 7233). The
`make` call is therefore bounded by the actual file size.

