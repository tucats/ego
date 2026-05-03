package caches

import (
	"net/http"
	"sort"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/caches"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/server/assets"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/server/services"
	"github.com/tucats/ego/util"
)

// GetCacheHandler is the HTTP handler for GET /admin/caches. It collects
// counts and item lists from every in-memory cache in the server and returns
// them as a single JSON response.
//
// In Ego, an HTTP handler receives three arguments:
//   - session: Ego's own request context (authenticated user, parsed parameters, …)
//   - w: the http.ResponseWriter used to write the HTTP response back to the client
//   - r: the *http.Request that describes what the client sent
//
// The return value is the HTTP status code (e.g. 200, 400).
func GetCacheHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	// Build the response struct, populating each field from the corresponding
	// cache package. util.MakeServerInfo attaches the server's own identity
	// (name, version, etc.) so the client knows which server replied.
	response := defs.CacheResponse{
		ServerInfo:         util.MakeServerInfo(session.ID),
		ServiceCount:       len(services.ServiceCache),
		ServiceCountLimit:  services.MaxCachedEntries,
		Items:              []defs.CachedItem{},
		AssetSize:          assets.GetAssetCacheSize(),
		AssetCount:         assets.GetAssetCacheCount(),
		Status:             http.StatusOK,
		UserItemsCount:     caches.Size(caches.UserCache),
		AuthorizationCount: caches.Size(caches.AuthCache),
		TokenCount:         caches.Size(caches.TokenCache),
		BlacklistCount:     caches.Size(caches.BlacklistCache),
		SchemaCount:        caches.Size(caches.SchemaCache),
		DSNCount:           caches.Size(caches.DSNCache),
		DebugCount:         caches.Size(caches.DebugSessionCache),
		RunCount:           caches.Size(caches.SymbolTableCache),
	}

	// Walk the service cache (compiled Ego service programs) and add an entry
	// to the Items list for each one. The "for k, v := range map" idiom gives
	// you the key (k) and value (v) for each entry in the map.
	for k, v := range services.ServiceCache {
		response.Items = append(response.Items, defs.CachedItem{
			Name:     k,
			LastUsed: v.Age,
			Count:    v.Count,
			Size:     v.Size,
			Class:    defs.ServiceCacheClass})
	}

	// Walk the asset cache (static files like JS, CSS, images) and do the same.
	for k, v := range assets.AssetCache {
		response.Items = append(response.Items, defs.CachedItem{
			Name:     k,
			LastUsed: v.LastUsed,
			Count:    v.Count,
			Size:     v.Size,
			Class:    defs.AssetCacheClass})
	}

	// Sort the results. By default, the array is sorted by the URL which is the path to the
	// cached objects. Other sort orders are supported, "count" and "time". The sort order
	// keyword names are case-insensitive.
	sortBy := "url"
	if session.Parameters["order-by"] != nil {
		sortBy = strings.ToLower(session.Parameters["order-by"][0])
	}

	// sort.Slice sorts a slice in-place using a caller-supplied "less" function.
	// The function receives two indices (i, j) and must return true when element
	// i should come before element j.
	switch sortBy {
	case "class":
		sort.Slice(response.Items, func(i, j int) bool {
			return response.Items[i].Class < response.Items[j].Class
		})
	case "url", "name", "path":
		sort.Slice(response.Items, func(i, j int) bool {
			return response.Items[i].Name < response.Items[j].Name
		})

	case "count", "hits":
		// Descending order — most-used entries first.
		sort.Slice(response.Items, func(i, j int) bool {
			return response.Items[i].Count > response.Items[j].Count
		})

	case "time", "age", "last-used":
		// Descending order — most recently used entries first.
		sort.Slice(response.Items, func(i, j int) bool {
			return response.Items[i].LastUsed.After(response.Items[j].LastUsed)
		})

	default:
		return util.ErrorResponse(w, session.ID, i18n.T("error.sort.order.invalid", ui.A{"order": sortBy}), http.StatusBadRequest)
	}

	// Set the Content-Type header so the client knows the response body is
	// Ego's cache media type, then serialize the response struct to JSON and
	// write it to the response writer.
	w.Header().Add(defs.ContentTypeHeader, defs.CacheMediaType)
	b := util.WriteJSON(w, response, &session.ResponseLength)

	// If the REST logger is enabled, record the full response body so that
	// it appears in the server log for debugging.
	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}
