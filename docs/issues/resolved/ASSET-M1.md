# ASSET-M1 — Wrong `Content-Range` header for open-ended ranges

**Affected file:** `server/assets/handler.go:176` — `AssetsHandler()`

**Description:**  
When a client sends `Range: bytes=N-`, the `end` variable in `AssetsHandler`
remains `EndOfData` (= `math.MaxInt64`) after parsing, because no explicit end
byte was specified. The handler then wrote this unmodified value directly into
the `Content-Range` response header: `bytes N-9223372036854775807/1000`. This
violates RFC 7233 §4.2, which requires the last-byte-pos in the Content-Range
header to reflect the actual last byte sent (i.e. `totalSize - 1`). Clients
that strictly parse the Content-Range value may reject or mishandle the
response.

**Recommendation:**  
After `Loader` returns `totalSize`, clamp `end` to `totalSize - 1` before
formatting the Content-Range header.

**Resolution (April 2026):**  
A `reportEnd` local variable is computed from `end`, clamped to `totalSize - 1`
when `end == EndOfData || end >= totalSize`, and used in the `Content-Range`
header format string. The handler's own `end` variable is not mutated.

