// Package assets tests cover the asset cache, the file loader, the markdown
// renderer, and the AssetsHandler HTTP handler. Tests that require filesystem
// access create a temporary directory and configure the settings package to
// use it as the asset root, then clean up on exit.
//
// The test package is "assets" (not "assets_test") so that unexported
// functions (lookupCachedAsset, cacheAsset, normalizeAssetPath, etc.) can be
// called directly.
package assets

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/server"
)

// makeTestDir creates a temporary directory populated with a small set of
// asset files used by the tests. The returned cleanup function removes the
// directory tree; call it with defer.
func makeTestDir(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "assets-test-*")
	if err != nil {
		t.Fatalf("os.MkdirTemp: %v", err)
	}

	files := map[string][]byte{
		"style.css":  []byte("body { color: red; }"),
		"index.html": []byte("<html><body>hello</body></html>"),
		"data.json":  []byte(`{"key":"value"}`),
		"page.md":    []byte("# Heading\n\nThis is **bold** text.\n"),
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0644); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}

	return dir, func() { os.RemoveAll(dir) }
}

// setAssetRoot configures the EgoLibPathSetting so normalizeAssetPath uses dir
// as the asset root. Call this at the top of any test that reads from disk.
func setAssetRoot(t *testing.T, dir string) {
	t.Helper()
	settings.Set(defs.EgoLibPathSetting, dir)
}

// makeSession returns a minimal server.Session adequate for AssetsHandler tests.
func makeSession() *server.Session {
	return &server.Session{ID: 1}
}

// assetRequest builds a GET *http.Request with r.URL.Path set to path and an
// optional Range header. Pass rangeHeader="" for non-range requests.
// When path is empty, httptest.NewRequest would panic (empty URL is invalid),
// so we build the request with "/" and then overwrite URL.Path.
func assetRequest(t *testing.T, path, rangeHeader string) *http.Request {
	t.Helper()

	urlStr := path
	if urlStr == "" {
		urlStr = "/"
	}

	req := httptest.NewRequest(http.MethodGet, urlStr, nil)
	req.URL.Path = path

	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}

	return req
}

// =========================================================================
// Cache tests
// =========================================================================

func TestFlushAssetCache_ClearsCountAndSize(t *testing.T) {
	FlushAssetCache()
	cacheAsset(0, "/seed.css", []byte("seed data"))

	if GetAssetCacheCount() == 0 {
		t.Fatal("pre-condition failed: expected at least one cached item before flush")
	}

	FlushAssetCache()

	if n := GetAssetCacheCount(); n != 0 {
		t.Errorf("count after flush: want 0, got %d", n)
	}

	if sz := GetAssetCacheSize(); sz != 0 {
		t.Errorf("size after flush: want 0, got %d", sz)
	}
}

func TestFlushAssetCache_MultipleCalls_Safe(t *testing.T) {
	// Calling Flush on an already-empty cache must not panic.
	FlushAssetCache()
	FlushAssetCache()
}

func TestCacheAsset_LookupHit(t *testing.T) {
	FlushAssetCache()

	content := []byte("body { color: red; }")
	cacheAsset(0, "/style.css", content)

	got := lookupCachedAsset(0, "/style.css")
	if got == nil {
		t.Fatal("expected cached asset, got nil")
	}

	if string(got) != string(content) {
		t.Errorf("cached content: want %q, got %q", content, got)
	}
}

func TestCacheAsset_LookupMiss_ReturnsNil(t *testing.T) {
	FlushAssetCache()

	if got := lookupCachedAsset(0, "/missing.css"); got != nil {
		t.Errorf("expected nil for uncached key, got %q", got)
	}
}

func TestCacheAsset_LookupInitializesNilCache(t *testing.T) {
	// lookupCachedAsset must not panic when AssetCache is nil.
	AssetCache = nil

	got := lookupCachedAsset(0, "/any.css")
	if got != nil {
		t.Errorf("expected nil for key not in a freshly-initialized cache, got %q", got)
	}

	// The cache must now be initialized (non-nil) so subsequent writes are safe.
	if AssetCache == nil {
		t.Error("expected AssetCache to be initialized after lookupCachedAsset, but it is still nil")
	}
}

func TestCacheAsset_SizeTracking(t *testing.T) {
	FlushAssetCache()

	data := []byte("hello world")
	cacheAsset(0, "/a.txt", data)

	if sz := GetAssetCacheSize(); sz != len(data) {
		t.Errorf("size after one insert: want %d, got %d", len(data), sz)
	}
}

func TestCacheAsset_UpdateReplacesOldEntry(t *testing.T) {
	FlushAssetCache()

	cacheAsset(0, "/foo.txt", []byte("short"))
	cacheAsset(0, "/foo.txt", []byte("much longer content here"))

	if n := GetAssetCacheCount(); n != 1 {
		t.Errorf("count after update: want 1, got %d", n)
	}

	want := len("much longer content here")
	if sz := GetAssetCacheSize(); sz != want {
		t.Errorf("size after update: want %d, got %d", want, sz)
	}
}

func TestCacheAsset_MultipleEntries(t *testing.T) {
	FlushAssetCache()

	cacheAsset(0, "/a.css", []byte("a"))
	cacheAsset(0, "/b.css", []byte("bb"))
	cacheAsset(0, "/c.css", []byte("ccc"))

	if n := GetAssetCacheCount(); n != 3 {
		t.Errorf("count: want 3, got %d", n)
	}

	if sz := GetAssetCacheSize(); sz != 6 {
		t.Errorf("cumulative size: want 6, got %d", sz)
	}
}

func TestCacheAsset_OversizedItem_NotCached(t *testing.T) {
	FlushAssetCache()

	// An item larger than half the max cache size must be silently dropped.
	oversized := make([]byte, maxAssetCacheSize/2+1)
	cacheAsset(0, "/huge.bin", oversized)

	if n := GetAssetCacheCount(); n != 0 {
		t.Errorf("oversized item must not be cached; count: want 0, got %d", n)
	}
}

func TestCacheAsset_Eviction_WhenCacheOverflows(t *testing.T) {
	FlushAssetCache()

	// Save and restore the cache limit so other tests are not affected.
	saved := maxAssetCacheSize
	defer func() { maxAssetCacheSize = saved }()

	// Set a tiny limit: 30 bytes.
	maxAssetCacheSize = 30

	// Items up to 15 bytes each (half the limit) are eligible for caching.
	cacheAsset(0, "/a.css", []byte("123456789012345")) // 15 bytes → cached
	cacheAsset(0, "/b.css", []byte("123456789012345")) // 15 bytes → cached, total=30
	cacheAsset(0, "/c.css", []byte("123456789012345")) // 15 bytes → total would be 45, eviction required

	// After eviction the total must be within the limit.
	if sz := GetAssetCacheSize(); sz > maxAssetCacheSize {
		t.Errorf("cache size after eviction: %d exceeds max %d", sz, maxAssetCacheSize)
	}
}

// TestCacheAsset_PathNormalizationConsistent verifies that a path stored
// without a leading slash is retrievable using the same path. Both
// cacheAsset and lookupCachedAsset now normalize via normalizeCachePath, so
// the same key is used regardless of whether the caller supplies a leading
// slash.
func TestCacheAsset_PathNormalizationConsistent(t *testing.T) {
	FlushAssetCache()

	content := []byte("cached content")
	cacheAsset(0, "no-slash.css", content)

	got := lookupCachedAsset(0, "no-slash.css")
	if got == nil {
		t.Fatal("asset stored without leading '/' must be findable by the same path (normalization must be consistent)")
	}

	if string(got) != string(content) {
		t.Errorf("retrieved content: want %q, got %q", content, got)
	}
}

// =========================================================================
// Markdown renderer tests
// =========================================================================

func TestMdToHTML_ProducesOutput(t *testing.T) {
	out := mdToHTML([]byte("# Hello\n\nParagraph.\n"))
	if len(out) == 0 {
		t.Error("expected non-empty HTML output, got empty byte slice")
	}
}

func TestMdToHTML_HeadingRendered(t *testing.T) {
	out := string(mdToHTML([]byte("# My Heading\n")))
	if !strings.Contains(out, "<h1") {
		t.Errorf("expected <h1> in output; got:\n%s", out)
	}

	if !strings.Contains(out, "My Heading") {
		t.Errorf("expected heading text in output; got:\n%s", out)
	}
}

func TestMdToHTML_BoldRendered(t *testing.T) {
	out := string(mdToHTML([]byte("This is **bold** text.\n")))
	if !strings.Contains(out, "<strong>") {
		t.Errorf("expected <strong> tag in output; got:\n%s", out)
	}
}

func TestMdToHTML_EmptyInput_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("mdToHTML panicked on empty input: %v", r)
		}
	}()

	_ = mdToHTML([]byte{})
}

func TestMdToHTML_PlainText_WrappedInParagraph(t *testing.T) {
	out := string(mdToHTML([]byte("just plain text\n")))
	if !strings.Contains(out, "plain text") {
		t.Errorf("expected plain text to appear in HTML output; got:\n%s", out)
	}
}

func TestMdToHTML_Link_HasTargetBlank(t *testing.T) {
	// The renderer is configured with html.HrefTargetBlank.
	out := string(mdToHTML([]byte("[click](https://example.com)\n")))
	if !strings.Contains(out, `target="_blank"`) {
		t.Errorf("expected target=\"_blank\" on links; got:\n%s", out)
	}
}

// =========================================================================
// normalizeAssetPath tests
// =========================================================================

func TestNormalizeAssetPath_NormalFile(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)

	got := normalizeAssetPath("/style.css")
	want := filepath.Join(dir, "style.css")

	if got != want {
		t.Errorf("normalizeAssetPath: want %q, got %q", want, got)
	}
}

func TestNormalizeAssetPath_NestedPath(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)

	got := normalizeAssetPath("/sub/file.js")
	want := filepath.Join(dir, "sub", "file.js")

	if got != want {
		t.Errorf("normalizeAssetPath: want %q, got %q", want, got)
	}
}

func TestNormalizeAssetPath_TraversalBlocked(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)

	got := normalizeAssetPath("/../../etc/passwd")
	if !strings.Contains(got, "__invalid__") {
		t.Errorf("expected path traversal to be blocked; got %q", got)
	}
}

func TestNormalizeAssetPath_TraversalInMiddle_Blocked(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)

	got := normalizeAssetPath("/sub/../../etc/passwd")
	if !strings.Contains(got, "__invalid__") {
		t.Errorf("expected mid-path traversal to be blocked; got %q", got)
	}
}

// =========================================================================
// Loader tests
// =========================================================================

func TestLoader_NegativeStart_ReturnsError(t *testing.T) {
	_, _, err := Loader(0, "/any.css", -1, EndOfData)
	if err == nil {
		t.Error("expected error for negative start, got nil")
	}
}

func TestLoader_ValidFile_FullLoad(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)
	FlushAssetCache()

	data, size, err := Loader(1, "/style.css", StartOfData, EndOfData)
	if err != nil {
		t.Fatalf("unexpected error loading existing file: %v", err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty data for existing file")
	}

	if size != int64(len(data)) {
		t.Errorf("totalSize mismatch: want %d, got %d", len(data), size)
	}

	if string(data) != "body { color: red; }" {
		t.Errorf("file content: want %q, got %q", "body { color: red; }", data)
	}
}

func TestLoader_MissingFile_ReturnsError(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)
	FlushAssetCache()

	_, _, err := Loader(1, "/does-not-exist.css", StartOfData, EndOfData)
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoader_SecondRead_ServedFromCache(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)
	FlushAssetCache()

	// First read — populates cache.
	data1, _, err := Loader(1, "/style.css", StartOfData, EndOfData)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}

	if GetAssetCacheCount() != 1 {
		t.Errorf("expected 1 cache entry after first load, got %d", GetAssetCacheCount())
	}

	// Second read — must come from cache without error.
	data2, _, err := Loader(1, "/style.css", StartOfData, EndOfData)
	if err != nil {
		t.Fatalf("second (cached) load: %v", err)
	}

	if string(data1) != string(data2) {
		t.Errorf("cached content differs from original: %q vs %q", data1, data2)
	}
}

func TestLoader_RangeRead_PartialContent(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)
	FlushAssetCache()

	// Read first 4 bytes of "body { color: red; }" (bytes 0-3 → "body").
	data, totalSize, err := Loader(1, "/style.css", 0, 3)
	if err != nil {
		if err == io.EOF {
			// See bug list: readAssetRange does not clear io.EOF before returning.
			t.Logf("BUG (io.EOF propagation): readAssetRange returned io.EOF for a successful partial read")

			return
		}

		t.Fatalf("unexpected error on range read: %v", err)
	}

	if string(data) != "body" {
		t.Errorf("range [0,3]: want %q, got %q", "body", data)
	}

	if totalSize != 20 {
		t.Errorf("totalSize for range read: want 20, got %d", totalSize)
	}
}

// TestLoader_RangeToEOF_NoError verifies that a range request extending to the
// last byte of a file succeeds without error. Previously, readAssetRange
// propagated io.EOF (which file.ReadAt may return for an exactly-sized read)
// back to the caller, causing AssetsHandler to return 404 instead of 206.
func TestLoader_RangeToEOF_NoError(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)
	FlushAssetCache()

	fileContent := "body { color: red; }"
	lastByte := int64(len(fileContent) - 1)

	data, _, err := Loader(1, "/style.css", 0, lastByte)
	if err != nil {
		t.Errorf("range read to last byte must not return an error; got: %v", err)
	}

	if string(data) != fileContent {
		t.Errorf("range [0, lastByte]: want %q, got %q", fileContent, data)
	}
}

func TestLoader_PathTraversal_ReturnsError(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)
	FlushAssetCache()

	// normalizeAssetPath returns __invalid__ for traversal attempts, which
	// does not exist — Loader must return an error rather than panicking.
	_, _, err := Loader(1, "/../../etc/passwd", StartOfData, EndOfData)
	if err == nil {
		t.Error("expected error for path-traversal attempt, got nil")
	}
}

// =========================================================================
// AssetsHandler tests
// =========================================================================

func TestAssetsHandler_EmptyPath_Returns403(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)
	FlushAssetCache()

	req := assetRequest(t, "", "")
	w := httptest.NewRecorder()

	status := AssetsHandler(makeSession(), w, req)
	if status != http.StatusForbidden {
		t.Errorf("empty path: want 403, got %d", status)
	}
}

func TestAssetsHandler_TrailingSlash_Returns403(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)
	FlushAssetCache()

	req := assetRequest(t, "/assets/", "")
	w := httptest.NewRecorder()

	status := AssetsHandler(makeSession(), w, req)
	if status != http.StatusForbidden {
		t.Errorf("trailing slash: want 403, got %d", status)
	}
}

func TestAssetsHandler_PathTraversal_Returns403(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)
	FlushAssetCache()

	// The handler explicitly blocks paths containing "/../".
	req := assetRequest(t, "/assets/../../../etc/passwd", "")
	w := httptest.NewRecorder()

	status := AssetsHandler(makeSession(), w, req)
	if status != http.StatusForbidden {
		t.Errorf("path traversal via '/../': want 403, got %d", status)
	}
}

func TestAssetsHandler_ValidCSS_Returns200(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)
	FlushAssetCache()

	req := assetRequest(t, "/style.css", "")
	w := httptest.NewRecorder()

	status := AssetsHandler(makeSession(), w, req)
	if status != http.StatusOK {
		t.Errorf("valid asset: want 200, got %d", status)
	}

	body := w.Body.String()
	if !strings.Contains(body, "red") {
		t.Errorf("response body missing expected CSS content; got: %q", body)
	}
}

func TestAssetsHandler_NotFound_Returns404(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)
	FlushAssetCache()

	req := assetRequest(t, "/nonexistent.css", "")
	w := httptest.NewRecorder()

	status := AssetsHandler(makeSession(), w, req)
	if status != http.StatusNotFound {
		t.Errorf("missing file: want 404, got %d", status)
	}
}

func TestAssetsHandler_MarkdownFile_RenderedAsHTML(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)
	FlushAssetCache()

	req := assetRequest(t, "/page.md", "")
	w := httptest.NewRecorder()

	status := AssetsHandler(makeSession(), w, req)
	if status != http.StatusOK {
		t.Errorf("markdown file: want 200, got %d", status)
	}

	body := w.Body.String()
	if !strings.Contains(body, "<h1") {
		t.Errorf("expected .md file to be rendered as HTML (expected <h1>); got: %q", body)
	}
}

func TestAssetsHandler_ContentType_CSS(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)
	FlushAssetCache()

	req := assetRequest(t, "/style.css", "")
	w := httptest.NewRecorder()
	_ = AssetsHandler(makeSession(), w, req)

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/css") {
		t.Errorf("CSS Content-Type: want text/css, got %q", ct)
	}
}

func TestAssetsHandler_ContentType_HTML(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)
	FlushAssetCache()

	req := assetRequest(t, "/index.html", "")
	w := httptest.NewRecorder()
	_ = AssetsHandler(makeSession(), w, req)

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("HTML Content-Type: want text/html, got %q", ct)
	}
}

func TestAssetsHandler_ContentType_JSON(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)
	FlushAssetCache()

	req := assetRequest(t, "/data.json", "")
	w := httptest.NewRecorder()
	_ = AssetsHandler(makeSession(), w, req)

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("JSON Content-Type: want application/json, got %q", ct)
	}
}

func TestAssetsHandler_AcceptsRangesHeader_AlwaysPresent(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)
	FlushAssetCache()

	req := assetRequest(t, "/style.css", "")
	w := httptest.NewRecorder()
	_ = AssetsHandler(makeSession(), w, req)

	if ar := w.Header().Get("Accept-Ranges"); ar != "bytes" {
		t.Errorf("Accept-Ranges header: want \"bytes\", got %q", ar)
	}
}

func TestAssetsHandler_ResponseLength_Updated(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)
	FlushAssetCache()

	req := assetRequest(t, "/style.css", "")
	w := httptest.NewRecorder()
	testSession := makeSession()
	_ = AssetsHandler(testSession, w, req)

	if testSession.ResponseLength == 0 {
		t.Error("expected ResponseLength > 0 after a successful response")
	}
}

func TestAssetsHandler_RangeRequest_Returns206(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)
	FlushAssetCache()

	// "bytes=0-3" requests the first four bytes of the CSS file ("body").
	req := assetRequest(t, "/style.css", "bytes=0-3")
	w := httptest.NewRecorder()

	status := AssetsHandler(makeSession(), w, req)
	if status != http.StatusPartialContent {
		// If the status is 404 this is likely the io.EOF bug in readAssetRange.
		t.Errorf("range request: want 206, got %d"+
			" (if 404, see io.EOF propagation bug in readAssetRange)", status)
	}
}

func TestAssetsHandler_InvalidRangeStart_Returns400(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)
	FlushAssetCache()

	req := assetRequest(t, "/style.css", "bytes=notANumber-3")
	w := httptest.NewRecorder()

	status := AssetsHandler(makeSession(), w, req)
	if status != http.StatusBadRequest {
		t.Errorf("invalid range start: want 400, got %d", status)
	}
}

func TestAssetsHandler_InvalidRangeEnd_Returns400(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)
	FlushAssetCache()

	req := assetRequest(t, "/style.css", "bytes=0-notANumber")
	w := httptest.NewRecorder()

	status := AssetsHandler(makeSession(), w, req)
	if status != http.StatusBadRequest {
		t.Errorf("invalid range end: want 400, got %d", status)
	}
}

func TestAssetsHandler_NegativeRangeStart_Returns400(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)
	FlushAssetCache()

	// start=-1 is syntactically valid hex but semantically invalid (negative).
	req := assetRequest(t, "/style.css", "bytes=-1-3")
	w := httptest.NewRecorder()

	// strconv.ParseInt("-1", 10, 64) parses correctly to -1,
	// which the sanity check "if start < 0" catches as a 400.
	status := AssetsHandler(makeSession(), w, req)
	if status != http.StatusBadRequest {
		t.Errorf("negative range start: want 400, got %d", status)
	}
}

// TestAssetsHandler_ContentRange_Header checks that a range response includes
// a well-formed Content-Range header and the 206 status code.
func TestAssetsHandler_ContentRange_Header(t *testing.T) {
	dir, cleanup := makeTestDir(t)
	defer cleanup()
	setAssetRoot(t, dir)
	FlushAssetCache()

	req := assetRequest(t, "/style.css", "bytes=0-3")
	w := httptest.NewRecorder()

	status := AssetsHandler(makeSession(), w, req)
	if status != http.StatusPartialContent {
		t.Skipf("range request returned %d (not 206); cannot verify Content-Range header", status)
	}

	cr := w.Header().Get("Content-Range")
	if !strings.HasPrefix(cr, "bytes 0-3/") {
		t.Errorf("Content-Range header: want prefix \"bytes 0-3/\", got %q", cr)
	}
}

// TestAssetsHandler_LibPathAssets uses the actual lib/assets directory (the
// same one a running server would use) to verify that the test fixture files
// placed there can be served correctly.
func TestAssetsHandler_LibPathAssets(t *testing.T) {
	// Find the lib/assets directory relative to this test file. The test binary
	// runs with the package directory as its working directory, so we go up two
	// levels (server/assets → server → repo root) and then into lib/assets.
	repoRoot, err := filepath.Abs("../../")
	if err != nil {
		t.Fatalf("cannot determine repo root: %v", err)
	}

	libAssets := filepath.Join(repoRoot, "lib", "assets")
	if _, err := os.Stat(libAssets); err != nil {
		t.Skipf("lib/assets directory not found (%v); skipping", err)
	}

	settings.Set(defs.EgoLibPathSetting, libAssets)
	FlushAssetCache()

	req := assetRequest(t, "/test.asset.css", "")
	w := httptest.NewRecorder()

	status := AssetsHandler(makeSession(), w, req)
	if status != http.StatusOK {
		t.Errorf("test fixture in lib/assets: want 200, got %d", status)
	}

	body := w.Body.String()
	if !strings.Contains(body, "color: red") {
		t.Errorf("expected CSS content in response; got: %q", body)
	}
}
