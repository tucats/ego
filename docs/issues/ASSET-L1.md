# ASSET-L1 — Double `os.ReadFile` call in `readAssetFile`

**Affected file:** `server/assets/handler.go:330,356` — `readAssetFile()`

**Description:**  
`os.ReadFile(fn)` was called twice: once at line 330 (result captured in `data`
and `err`), then again at line 356 as the function's return value. The first
result was used only for logging and then discarded; the second read was what
actually propagated to the caller. If the file was modified or deleted between
the two reads, the caller received different content (or an error) than what
was logged. In addition, it doubled the I/O cost of every uncached asset load.

**Recommendation:**  
Return the `data` and `err` captured by the first read.

**Resolution (April 2026):**  
`return os.ReadFile(fn)` at line 356 replaced with `return data, err`.

