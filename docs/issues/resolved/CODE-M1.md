# CODE-M1 — No request body size limit on `POST /admin/run`

**Affected file:** `server/admin/run.go:163` — `RunCodeHandler()`

```go
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
```

**Description:**  
The request body is decoded with no preceding call to `http.MaxBytesReader`.
Any user who holds the `ego.server.admin` or `ego.code` permission can POST an
arbitrarily large body. The full body is buffered before the JSON decoder
returns, so a multi-megabyte `Code` field will be compiled and executed (or
at least compiled). A sustained flood of large requests can exhaust server
memory without triggering the global body-size limit applied at the transport
layer by HTTP-H1, because `RunCodeHandler` re-reads `r.Body` directly rather
than consuming the pre-read `session.Body` buffer used by most other handlers.

**Recommendation:**  
Wrap the body before decoding, and add a post-decode length check on the `Code`
field:

```go
const maxRunBodyBytes = 1 << 18  // 256 KiB — generous for any plausible script
r.Body = http.MaxBytesReader(w, r.Body, maxRunBodyBytes)
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    return util.ErrorResponse(w, session.ID, err.Error(), http.StatusRequestEntityTooLarge)
}
if len(req.Code) > maxRunCodeBytes {
    return util.ErrorResponse(w, session.ID, "code too large", http.StatusRequestEntityTooLarge)
}
```

**Resolution (April 2026):**  
`RunCodeHandler` now wraps `r.Body` with `http.MaxBytesReader` (256 KiB limit)
before JSON decoding. A `*http.MaxBytesError` from the decoder returns 413;
other decode errors return 400. A post-decode `len(req.Code) > maxRunCodeBytes`
check also returns 413 for oversized code fields.

