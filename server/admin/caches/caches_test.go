package caches

// Tests for the three HTTP handlers in the server/admin/caches package:
//
//   GetCacheHandler   – GET  /admin/caches
//   PurgeCacheHandler – DELETE /admin/caches
//   SetCacheSizeHandler – POST /admin/caches
//
// Each test constructs a minimal server.Session, uses net/http/httptest to
// capture the HTTP response, and asserts on the status code and/or the
// decoded JSON response body.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tucats/ego/caches"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/router"
	"github.com/tucats/ego/server/assets"
	"github.com/tucats/ego/server/services"
)

// makeSession builds a minimal server.Session sufficient for these handlers.
// Only the fields that the cache handlers actually read are populated:
//   - ID is the session identifier used in logging and the ServerInfo header.
//   - Parameters holds URL query parameters (e.g. "order-by", "class").
func makeSession(params map[string][]string) *router.Session {
	if params == nil {
		params = map[string][]string{}
	}

	return &router.Session{
		ID:         1,
		Parameters: params,
	}
}

// newRequest is a convenience wrapper that creates a new *http.Request with an
// optional JSON body. Pass nil body for requests that carry no payload (GET,
// DELETE).
func newRequest(t *testing.T, method string, body []byte) *http.Request {
	t.Helper()

	var bodyReader *bytes.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}

	req, err := http.NewRequest(method, "/admin/caches", bodyReader)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}

	return req
}

// decodeResponse decodes the JSON response body written to a ResponseRecorder
// into a CacheResponse struct.
func decodeResponse(t *testing.T, rr *httptest.ResponseRecorder) defs.CacheResponse {
	t.Helper()

	var resp defs.CacheResponse

	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal: %v\nbody was: %s", err, rr.Body.String())
	}

	return resp
}

// resetCaches puts all global cache state back to a known baseline so that
// tests do not interfere with each other.
func resetCaches() {
	services.FlushServiceCache()
	assets.FlushAssetCache()
	caches.PurgeAll()

	services.MaxCachedEntries = 20
}

// ---------------------------------------------------------------------------
// GetCacheHandler tests
// ---------------------------------------------------------------------------

func TestGetCacheHandler_ReturnsOK(t *testing.T) {
	// A plain GET with no special state: expect 200 and a parseable body.
	resetCaches()

	rr := httptest.NewRecorder()
	status := GetCacheHandler(makeSession(nil), rr, newRequest(t, http.MethodGet, nil))

	if status != http.StatusOK {
		t.Errorf("expected status 200, got %d", status)
	}

	if rr.Code != http.StatusOK {
		t.Errorf("expected recorder code 200, got %d", rr.Code)
	}

	resp := decodeResponse(t, rr)
	if resp.Status != http.StatusOK {
		t.Errorf("expected body status 200, got %d", resp.Status)
	}
}

func TestGetCacheHandler_ServiceCountReflectsCache(t *testing.T) {
	// Seed the service cache with one entry and confirm it appears in the count.
	resetCaches()

	services.ServiceCache["test-endpoint"] = &services.CachedCompilationUnit{}

	rr := httptest.NewRecorder()
	GetCacheHandler(makeSession(nil), rr, newRequest(t, http.MethodGet, nil))

	resp := decodeResponse(t, rr)
	if resp.ServiceCount != 1 {
		t.Errorf("expected ServiceCount 1, got %d", resp.ServiceCount)
	}
}

func TestGetCacheHandler_ServiceCountLimitReflectsGlobal(t *testing.T) {
	resetCaches()

	services.MaxCachedEntries = 42

	rr := httptest.NewRecorder()
	GetCacheHandler(makeSession(nil), rr, newRequest(t, http.MethodGet, nil))

	resp := decodeResponse(t, rr)
	if resp.ServiceCountLimit != 42 {
		t.Errorf("expected ServiceCountLimit 42, got %d", resp.ServiceCountLimit)
	}
}

func TestGetCacheHandler_DefaultSortByURL(t *testing.T) {
	// With no order-by parameter the handler sorts by URL (name). The request
	// should succeed — we are mainly testing that the default path doesn't error.
	resetCaches()

	services.ServiceCache["b-endpoint"] = &services.CachedCompilationUnit{}
	services.ServiceCache["a-endpoint"] = &services.CachedCompilationUnit{}

	rr := httptest.NewRecorder()
	status := GetCacheHandler(makeSession(nil), rr, newRequest(t, http.MethodGet, nil))

	if status != http.StatusOK {
		t.Errorf("expected 200, got %d", status)
	}

	resp := decodeResponse(t, rr)
	if len(resp.Items) < 2 {
		t.Fatalf("expected at least 2 items, got %d", len(resp.Items))
	}

	// After sorting by name ascending, "a-endpoint" should come first.
	if resp.Items[0].Name != "a-endpoint" {
		t.Errorf("expected first item to be a-endpoint, got %q", resp.Items[0].Name)
	}
}

func TestGetCacheHandler_SortByCount(t *testing.T) {
	resetCaches()

	services.ServiceCache["low"] = &services.CachedCompilationUnit{Count: 1}
	services.ServiceCache["high"] = &services.CachedCompilationUnit{Count: 99}

	params := map[string][]string{"order-by": {"count"}}
	rr := httptest.NewRecorder()
	status := GetCacheHandler(makeSession(params), rr, newRequest(t, http.MethodGet, nil))

	if status != http.StatusOK {
		t.Errorf("expected 200, got %d", status)
	}

	resp := decodeResponse(t, rr)
	if len(resp.Items) < 2 {
		t.Fatalf("expected at least 2 items, got %d", len(resp.Items))
	}

	// Descending by count — "high" (99) should come before "low" (1).
	if resp.Items[0].Name != "high" {
		t.Errorf("expected first item to be 'high', got %q", resp.Items[0].Name)
	}
}

func TestGetCacheHandler_SortByClass(t *testing.T) {
	resetCaches()

	services.ServiceCache["svc"] = &services.CachedCompilationUnit{}

	params := map[string][]string{"order-by": {"class"}}
	rr := httptest.NewRecorder()
	status := GetCacheHandler(makeSession(params), rr, newRequest(t, http.MethodGet, nil))

	if status != http.StatusOK {
		t.Errorf("expected 200, got %d", status)
	}
}

func TestGetCacheHandler_InvalidSortOrder(t *testing.T) {
	// An unrecognized order-by value should produce a 400 response.
	resetCaches()

	params := map[string][]string{"order-by": {"bogus"}}
	rr := httptest.NewRecorder()
	status := GetCacheHandler(makeSession(params), rr, newRequest(t, http.MethodGet, nil))

	if status != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid sort order, got %d", status)
	}
}

func TestGetCacheHandler_SortOrderCaseInsensitive(t *testing.T) {
	// "COUNT" (uppercase) should be treated the same as "count".
	resetCaches()

	params := map[string][]string{"order-by": {"COUNT"}}
	rr := httptest.NewRecorder()
	status := GetCacheHandler(makeSession(params), rr, newRequest(t, http.MethodGet, nil))

	if status != http.StatusOK {
		t.Errorf("expected 200 for case-insensitive sort key, got %d", status)
	}
}

// ---------------------------------------------------------------------------
// PurgeCacheHandler tests
// ---------------------------------------------------------------------------

func TestPurgeCacheHandler_PurgesAll(t *testing.T) {
	// Populate several caches, then call purge with no class parameter.
	// After the purge every count should be zero.
	resetCaches()

	services.ServiceCache["svc"] = &services.CachedCompilationUnit{}

	rr := httptest.NewRecorder()
	status := PurgeCacheHandler(makeSession(nil), rr, newRequest(t, http.MethodDelete, nil))

	if status != http.StatusOK {
		t.Errorf("expected 200, got %d", status)
	}

	resp := decodeResponse(t, rr)
	if resp.ServiceCount != 0 {
		t.Errorf("expected ServiceCount 0 after full purge, got %d", resp.ServiceCount)
	}
}

func TestPurgeCacheHandler_PurgesServices(t *testing.T) {
	// Seed the service cache, then purge only the "services" class.
	resetCaches()

	services.ServiceCache["svc-a"] = &services.CachedCompilationUnit{}
	services.ServiceCache["svc-b"] = &services.CachedCompilationUnit{}

	params := map[string][]string{"class": {"services"}}
	rr := httptest.NewRecorder()
	status := PurgeCacheHandler(makeSession(params), rr, newRequest(t, http.MethodDelete, nil))

	if status != http.StatusOK {
		t.Errorf("expected 200, got %d", status)
	}

	resp := decodeResponse(t, rr)
	if resp.ServiceCount != 0 {
		t.Errorf("expected ServiceCount 0 after services purge, got %d", resp.ServiceCount)
	}
}

func TestPurgeCacheHandler_PurgesAssets(t *testing.T) {
	// The assets package exposes FlushAssetCache; verify that the "assets"
	// class name routes to it correctly.
	resetCaches()

	params := map[string][]string{"class": {"assets"}}
	rr := httptest.NewRecorder()
	status := PurgeCacheHandler(makeSession(params), rr, newRequest(t, http.MethodDelete, nil))

	if status != http.StatusOK {
		t.Errorf("expected 200, got %d", status)
	}

	// After flushing assets, AssetCount should be 0.
	resp := decodeResponse(t, rr)
	if resp.AssetCount != 0 {
		t.Errorf("expected AssetCount 0 after assets purge, got %d", resp.AssetCount)
	}
}

func TestPurgeCacheHandler_ClassNameCaseInsensitive(t *testing.T) {
	// "SERVICES" should be treated the same as "services".
	resetCaches()

	services.ServiceCache["svc"] = &services.CachedCompilationUnit{}

	params := map[string][]string{"class": {"SERVICES"}}
	rr := httptest.NewRecorder()
	status := PurgeCacheHandler(makeSession(params), rr, newRequest(t, http.MethodDelete, nil))

	if status != http.StatusOK {
		t.Errorf("expected 200, got %d", status)
	}

	resp := decodeResponse(t, rr)
	if resp.ServiceCount != 0 {
		t.Errorf("expected ServiceCount 0 after SERVICES purge, got %d", resp.ServiceCount)
	}
}

func TestPurgeCacheHandler_UnknownClassIsIgnored(t *testing.T) {
	// An unrecognized class name is silently ignored (falls through the switch).
	resetCaches()

	params := map[string][]string{"class": {"nonexistent"}}
	rr := httptest.NewRecorder()
	status := PurgeCacheHandler(makeSession(params), rr, newRequest(t, http.MethodDelete, nil))

	if status != http.StatusOK {
		t.Errorf("expected 200 for unknown class, got %d", status)
	}
}

func TestPurgeCacheHandler_MultipleClasses(t *testing.T) {
	// Supplying multiple class values in one request should purge each named cache.
	resetCaches()

	services.ServiceCache["svc"] = &services.CachedCompilationUnit{}

	params := map[string][]string{"class": {"services", "assets"}}
	rr := httptest.NewRecorder()
	status := PurgeCacheHandler(makeSession(params), rr, newRequest(t, http.MethodDelete, nil))

	if status != http.StatusOK {
		t.Errorf("expected 200, got %d", status)
	}

	resp := decodeResponse(t, rr)
	if resp.ServiceCount != 0 {
		t.Errorf("expected ServiceCount 0 after multi-class purge, got %d", resp.ServiceCount)
	}
}

// ---------------------------------------------------------------------------
// SetCacheSizeHandler tests
// ---------------------------------------------------------------------------

func TestSetCacheSizeHandler_UpdatesMaxEntries(t *testing.T) {
	// A valid JSON body with serviceSize should update MaxCachedEntries.
	resetCaches()

	body, _ := json.Marshal(defs.CacheResponse{ServiceCountLimit: 7})
	rr := httptest.NewRecorder()
	status := SetCacheSizeHandler(makeSession(nil), rr, newRequest(t, http.MethodPost, body))

	if status != http.StatusOK {
		t.Errorf("expected 200, got %d", status)
	}

	if services.MaxCachedEntries != 7 {
		t.Errorf("expected MaxCachedEntries 7, got %d", services.MaxCachedEntries)
	}
}

func TestSetCacheSizeHandler_ResponseReflectsNewLimit(t *testing.T) {
	// The response body should report the newly applied limit.
	resetCaches()

	body, _ := json.Marshal(defs.CacheResponse{ServiceCountLimit: 15})
	rr := httptest.NewRecorder()
	SetCacheSizeHandler(makeSession(nil), rr, newRequest(t, http.MethodPost, body))

	resp := decodeResponse(t, rr)
	if resp.ServiceCountLimit != 15 {
		t.Errorf("expected ServiceCountLimit 15 in response, got %d", resp.ServiceCountLimit)
	}
}

func TestSetCacheSizeHandler_RejectsBadJSON(t *testing.T) {
	// Malformed JSON should produce a 400 Bad Request response.
	resetCaches()

	body := []byte(`not valid json`)
	rr := httptest.NewRecorder()
	status := SetCacheSizeHandler(makeSession(nil), rr, newRequest(t, http.MethodPost, body))

	if status != http.StatusBadRequest {
		t.Errorf("expected 400 for bad JSON, got %d", status)
	}
}

func TestSetCacheSizeHandler_EmptyBodyIsInvalid(t *testing.T) {
	// An empty body is not valid JSON, so we expect a 400.
	resetCaches()

	rr := httptest.NewRecorder()
	status := SetCacheSizeHandler(makeSession(nil), rr, newRequest(t, http.MethodPost, nil))

	if status != http.StatusBadRequest {
		t.Errorf("expected 400 for empty body, got %d", status)
	}
}
