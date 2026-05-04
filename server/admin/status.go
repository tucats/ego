package admin

import (
	"net/http"
	"runtime"
	"sort"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/caches"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/assets"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/server/services"
	"github.com/tucats/ego/util"
)

// GetResourcesHandler is the HTTP handler for GET /admin/resources. It reports the
// server's current memory usage and cache information in a single call. This
// is currently primarily used by the dashboard webapp to reduce the number of
// round trips for it's status page. See the GetMemoryHandler() and GetCacheHandler()
// functions for more details on the operation of this code.
func GetResourcesHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	m := &runtime.MemStats{}
	runtime.ReadMemStats(m)

	response := defs.StatusResponse{
		ServerInfo:         util.MakeServerInfo(session.ID),
		Total:              int(m.TotalAlloc),
		Current:            int(m.HeapInuse),
		System:             int(m.Sys),
		Stack:              int(m.StackInuse),
		Objects:            int(m.HeapObjects),
		GCCount:            int(m.NumGC),
		ServiceCount:       len(services.ServiceCache),
		ServiceCountLimit:  services.MaxCachedEntries,
		Items:              []defs.CachedItem{},
		AssetSize:          assets.GetAssetCacheSize(),
		AssetCount:         assets.GetAssetCacheCount(),
		UserItemsCount:     caches.Size(caches.UserCache),
		AuthorizationCount: caches.Size(caches.AuthCache),
		TokenCount:         caches.Size(caches.TokenCache),
		BlacklistCount:     caches.Size(caches.BlacklistCache),
		SchemaCount:        caches.Size(caches.SchemaCache),
		DSNCount:           caches.Size(caches.DSNCache),
		DebugCount:         caches.Size(caches.DebugSessionCache),
		RunCount:           caches.Size(caches.SymbolTableCache),
		Status:             http.StatusOK,
	}

	// Walk the service cache (compiled Ego service programs) and add an entry
	// to the Items list for each one.
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

	// Sort the cached items alphabetically by URL
	sort.Slice(response.Items, func(i, j int) bool {
		return response.Items[i].Name < response.Items[j].Name
	})

	// Set the Content-Type header so clients know the response body contains
	// Ego status info rather than plain JSON.
	w.Header().Add(defs.ContentTypeHeader, defs.ResourcesMediaType)

	// util.WriteJSON serializes response to JSON, writes it to w, and returns
	// the raw bytes so we can log them below.  session.ResponseLength is
	// updated so the server can report how many bytes were sent.
	b := util.WriteJSON(w, response, &session.ResponseLength)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}
