# Asset Server — `server/assets` Package

This document explains how the asset endpoint works, how range requests are handled, how the in-memory cache operates, and how to use the `Loader` function from other Go code inside Ego. It is written for a developer who is new to the codebase.

---

## Table of Contents

1. [What this package does](#1-what-this-package-does)
2. [The HTTP endpoint](#2-the-http-endpoint)
   - [Registration](#registration)
   - [Request path rules](#request-path-rules)
   - [Content-Type mapping](#content-type-mapping)
   - [Markdown rendering](#markdown-rendering)
   - [Response headers](#response-headers)
3. [Range requests](#3-range-requests)
   - [How range requests work](#how-range-requests-work)
   - [Range request flow](#range-request-flow)
4. [The asset cache](#4-the-asset-cache)
   - [Cache key normalization](#cache-key-normalization)
   - [Cache eviction](#cache-eviction)
   - [Cache size limit](#cache-size-limit)
   - [Oversized assets](#oversized-assets)
   - [Managing the cache at runtime](#managing-the-cache-at-runtime)
5. [Configuration settings](#5-configuration-settings)
6. [JavaScript minification](#6-javascript-minification)
7. [Using `Loader` from Go code](#7-using-loader-from-go-code)
8. [Security and path confinement](#8-security-and-path-confinement)
9. [Diagnostic logging](#9-diagnostic-logging)
10. [Public API summary](#10-public-api-summary)

---

## 1. What this package does

The `assets` package serves static files (HTML, CSS, JavaScript, images, PDFs, Markdown, etc.) from the server's file system. It is the back-end for the `/assets/` HTTP endpoint that client-side HTML pages use to load resources like stylesheets, scripts, and images.

The package has three main responsibilities:

- **Serve files** — Read a file from disk and return it to the HTTP client with the correct `Content-Type` header.
- **Cache files** — Keep recently served files in memory so that repeated requests do not hit the disk.
- **Handle byte-range requests** — Return only a sub-range of a file, which browsers use for video and audio streaming.

---

## 2. The HTTP endpoint

### Registration

The endpoint is registered in the server's route table as:

```text
GET /assets/{item...}
```

Everything after `/assets/` becomes the path that is resolved against the asset root directory on the server's file system. For example, a request for `/assets/dashboard/style.css` looks for the file `style.css` inside the `dashboard/` subdirectory of the asset root.

The handler function is `AssetsHandler(session, w, r)`. It takes a `*server.Session` (which carries the session ID used for logging), a standard `http.ResponseWriter`, and an `*http.Request`. It returns an integer HTTP status code.

### Request path rules

Two types of path are rejected immediately with **403 Forbidden**:

| Condition | Why |
| ---------  --- |
| Path is empty or ends with `/` | Directory listing is not allowed. |
| Path contains `/../` | Prevents escaping the asset root via relative references. |

All other paths go through the file loader, which applies a second, stricter confinement check (see [Security and path confinement](#8-security-and-path-confinement)).

### Content-Type mapping

The response `Content-Type` header is set based on the file extension:

| Extension | Content-Type |
| --------- | ------------ |
| `.md` | `text/html` (rendered from Markdown) |
| `.html`, `.htm` | `text/html` |
| `.css` | `text/css` |
| `.js` | `text/javascript` |
| `.json` | `application/json` |
| `.txt`, `.text` | `application/text` |
| `.pdf` | `application/pdf` |
| `.mp4` | `video/mp4` |
| *(other)* | *(no Content-Type set; browser infers)* |

### Markdown rendering

If the requested path ends with `.md`, the raw Markdown bytes are automatically converted to a full HTML document before the response is written. The converter uses the `gomarkdown` library with the `CommonExtensions` and `AutoHeadingIDs` parser options and the `HrefTargetBlank` renderer flag (so all links open in a new tab). The resulting `Content-Type` is `text/html`.

### Response headers

Every successful asset response includes:

```text
Accept-Ranges: bytes
```

This tells the browser that byte-range requests are supported for this resource. When a range is requested, the response also includes `Content-Range` and `Content-Length` (see the next section).

---

## 3. Range requests

### How range requests work

The HTTP `Range` header (defined in [RFC 7233](https://datatracker.ietf.org/doc/html/rfc7233)) lets a client request only part of a file. Browsers use this for video and audio: rather than downloading an entire video before playing it, the browser asks for a small chunk, plays it, then asks for the next chunk.

A range request looks like this:

```text
GET /assets/video.mp4 HTTP/1.1
Range: bytes=0-1048575
```

That says "give me bytes 0 through 1,048,575 (the first 1 MB)." An open-ended range like `bytes=512-` means "from byte 512 to the end of the file."

### Range request flow

1. The handler parses the `Range` header and extracts the start and end byte positions.
2. If either position is not a valid integer, the handler returns **400 Bad Request**.
3. If `start < 0` or `end < start`, the handler returns **400 Bad Request**.
4. The file is loaded via `readAssetRange`, which seeks directly to `start` and reads only the requested bytes. This avoids loading the whole file into memory — important for large video files.
5. The end byte is clamped to `(file size − 1)` so open-ended ranges never over-read.
6. A successful range response carries:
   - **Status 206 Partial Content**
   - `Content-Range: bytes <start>-<end>/<total-size>`
   - `Content-Length: <bytes-returned>`

Full (non-range) requests return **200 OK** with the complete file and no `Content-Range` header.

> **Note for maintainers:** Range requests are handled by `readAssetRange` and bypass the cache. The cache is only populated for full (non-range) reads, so a later full request for the same asset will still benefit from the cache even if earlier range requests were made.

---

## 4. The asset cache

The cache is a process-wide, in-memory `map[string]AssetObject`. Each entry stores the file bytes, the access count, the size, and the time of the last access. All cache operations are protected by a mutex (`AssetMux`) so they are safe to call from concurrent request goroutines.

### Cache key normalization

Cache keys are the request path with a leading `/` ensured. For example, both `"style.css"` and `"/style.css"` map to the same cache entry `"/style.css"`. This normalization is applied consistently in both `cacheAsset` (when storing) and `lookupCachedAsset` (when retrieving) via the internal `normalizeCachePath` helper.

### Cache eviction

When a new entry would push the total cached bytes above `maxAssetCacheSize`, the cache evicts entries one at a time — oldest first (by `LastUsed` time) — until the total is back within the limit.

### Cache size limit

The default maximum cache size is **10 MB** (`10 * 1024 * 1024` bytes). This can be changed at runtime via the `ego server caches set-size` command or the `DELETE /admin/caches` and `POST /admin/caches` REST endpoints. The limit is stored in the package-level variable `maxAssetCacheSize`.

### Oversized assets

An individual asset larger than **half the maximum cache size** is never stored in the cache. It is read from disk on every request. This prevents a single large file (such as a video) from evicting everything else and filling the cache by itself.

The threshold is `maxAssetCacheSize / 2`. With the default 10 MB limit, any single asset larger than **5 MB** is always read from disk.

### Managing the cache at runtime

**From the command line:**

```sh
# Show current cache contents and sizes
ego server caches show

# Remove all entries from the asset cache
ego server caches flush

# Change the total cache size limit (bytes)
ego server caches set-size <bytes>
```

**From Go code (within the server):**

```go
assets.FlushAssetCache()          // remove all entries, reset size counter
n   := assets.GetAssetCacheCount() // number of entries currently cached
sz  := assets.GetAssetCacheSize()  // total bytes currently cached
```

---

## 5. Configuration settings

The package reads the following settings from the Ego configuration system (set with `ego config set <key>=<value>`):

| Setting key | Purpose | Default |
| ----------- | ------- | ------- |
| `ego.runtime.path.lib` | Explicit path to the asset root directory. When set, this takes precedence over the derived path. | *(not set)* |
| `ego.runtime.path` | Ego's root directory. The asset root is derived as `<ego.runtime.path>/lib` when `ego.runtime.path.lib` is not set. | *(not set)* |
| `ego.server.js.minify` | When `true`, `.js` files are minified on first load before being placed in the cache. | `false` |
| `ego.server.js.shortvarnames` | When `true` (and minification is enabled), local variable names are shortened to single characters. Only takes effect when `ego.server.js.minify` is also `true`. | `false` |

The asset root directory is resolved in this order:

1. Use `ego.runtime.path.lib` if set.
2. Otherwise use `<ego.runtime.path>/lib`.
3. If neither is set, the root defaults to `""`, which resolves relative to the current working directory. This should be avoided in production.

---

## 6. JavaScript minification

When `ego.server.js.minify` is enabled, `.js` files go through an in-process minifier on their first load. Minification removes comments and unnecessary whitespace. If `ego.server.js.shortvarnames` is also enabled, local variable names are compressed to short identifiers, reducing file size further.

The minification is applied **once**, before the result is stored in the cache. Subsequent requests for the same `.js` file are served from the cache and are not re-minified. If the cache is flushed, the next request re-reads the original file from disk and minifies it again.

The log entry `asset.minify` (visible with `--log ASSET`) reports the original size, minified size, bytes saved, and percentage reduction.

---

## 7. Using `Loader` from Go code

`Loader` is the package's public function for loading an asset file from within the server without going through the HTTP layer. Other server packages (such as the UI endpoint) use it to read assets programmatically.

### Signature

```go
func Loader(sessionID int, path string, start, end int64) (data []byte, totalSize int64, err error)
```

| Parameter | Description |
| --------- | ----------- |
| `sessionID` | An integer identifying the current request, used only for logging. Pass `0` if there is no session context. |
| `path` | The path to the asset relative to the asset root, e.g. `"/dashboard/style.css"`. |
| `start` | First byte to return. Use `assets.StartOfData` (= `0`) for a full read. |
| `end` | Last byte to return (inclusive). Use `assets.EndOfData` (= `math.MaxInt64`) for a full read. |

| Return value | Description |
| ------------ | ----------- |
| `data` | The bytes that were read (the requested range, or the full file). |
| `totalSize` | The total size of the file in bytes, regardless of how many bytes were returned. Useful for writing `Content-Range` headers. |
| `err` | Non-nil if the file could not be found or read. |

### Example — loading a full file

```go
import "github.com/tucats/ego/server/assets"

data, _, err := assets.Loader(session.ID, "/dashboard/config.json", assets.StartOfData, assets.EndOfData)
if err != nil {
    // file not found or read error
    return http.StatusNotFound
}
// use data ...
```

### Example — loading a byte range

```go
// Read bytes 1024 through 2047 of a large file
data, totalSize, err := assets.Loader(session.ID, "/video/intro.mp4", 1024, 2047)
if err != nil {
    return http.StatusInternalServerError
}
w.Header().Set("Content-Range", fmt.Sprintf("bytes 1024-2047/%d", totalSize))
w.WriteHeader(http.StatusPartialContent)
w.Write(data)
```

### Caching behavior

- A **full read** (`start = StartOfData`, `end = EndOfData`) first checks the cache. On a cache miss, the file is read from disk and stored in the cache for future calls.
- A **range read** (any other `start`/`end`) always reads directly from disk and is never stored in the cache. This prevents large partial reads from polluting the cache.
- If you call `Loader` from multiple goroutines concurrently, the cache operations are mutex-protected and safe.

---

## 8. Security and path confinement

Because the endpoint serves arbitrary paths supplied by HTTP clients, it must ensure that a malicious request cannot escape the asset root directory and read files from elsewhere on the server's file system (a "path traversal" attack).

The package uses a two-layer defense:

**Layer 1 — Early handler check:**
`AssetsHandler` immediately rejects any path that contains the substring `/../` and returns 403. This catches the most obvious traversal attempts before touching the file system.

**Layer 2 — `normalizeAssetPath` confinement check:**
`normalizeAssetPath` resolves the full canonical path with `filepath.Clean`, then verifies that the result is a child of the asset root. If the cleaned path does not start with `<root>/`, the function returns an impossible path (`<root>/__invalid__`) that will produce a "file not found" error on any read attempt. This catches subtler attacks, such as URL-encoded sequences that the HTTP layer decodes before the handler runs.

```text
Request: /assets/../../etc/passwd
                 ↓ Layer 1 check: contains "/../" → 403 Forbidden (immediate)

Request: /assets/%2e%2e/etc/passwd   (URL-encoded "..")
                 ↓ Layer 1 check: path after decoding = /assets/../etc/passwd → 403

Request: /assets/sub/..              (no "/../", but resolves above root)
                 ↓ Layer 1 check: passes (no "/../" substring)
                 ↓ Layer 2 (filepath.Clean): resolves to asset root itself → __invalid__ → 404
```

---

## 9. Diagnostic logging

The package writes log entries to the `ASSET` log class. Enable it with `--log ASSET` before any sub-command:

```sh
ego --log ASSET server start
```

Key log entries (message key → when it appears):

| Message key | When it appears |
| ----------- | -------------- |
| `asset.init` | Cache is flushed or first initialized. |
| `asset.loaded` | An asset was found in the cache. |
| `asset.not.found` | Cache miss (will attempt disk read). |
| `asset.read` | A file was read from disk in a server session. |
| `asset.read.local` | A file was read from disk outside a session (sessionID == 0). |
| `asset.read.range` | A byte-range was read from disk. |
| `asset.saved` | A file was added to the cache. |
| `asset.purged` | An entry was evicted to make room for a new one. |
| `asset.too.large` | A file exceeded the per-item size limit and was not cached. |
| `asset.minify` | A `.js` file was minified before caching. |
| `asset.load.error` | A disk read failed (path and error logged server-side only). |

---

## 10. Public API summary

These are the exported symbols in the package:

```go
// Constants for Loader start/end parameters.
const StartOfData = int64(0)
const EndOfData   = int64(math.MaxInt64)
const MaxAssetSize = 2 * 1024 * 1024  // 2 MB

// AssetObject is one entry in the cache.
type AssetObject struct {
    Data     []byte
    Count    int
    Size     int
    LastUsed time.Time
}

// AssetCache is the live cache map. Direct access is possible but
// AssetMux must be held. Prefer the accessor functions below.
var AssetCache map[string]AssetObject
var AssetMux   sync.Mutex

// AssetsHandler is the HTTP handler registered at GET /assets/{item...}.
func AssetsHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int

// Loader loads an asset file, using the cache for full reads.
func Loader(sessionID int, path string, start, end int64) ([]byte, int64, error)

// FlushAssetCache empties the cache and resets the byte counter.
func FlushAssetCache()

// GetAssetCacheSize returns the total bytes currently in the cache.
func GetAssetCacheSize() int

// GetAssetCacheCount returns the number of entries currently in the cache.
func GetAssetCacheCount() int
```
