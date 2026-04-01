package caches

import (
	"net/http"
	"sort"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/caches"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/assets"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/server/services"
	"github.com/tucats/ego/util"
)

// GetCacheHandler is the cache endpoint handler for retrieving the cache status from the server.
func GetCacheHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
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
	}

	for k, v := range services.ServiceCache {
		response.Items = append(response.Items, defs.CachedItem{Name: k, LastUsed: v.Age, Count: v.Count, Class: defs.ServiceCacheClass})
	}

	for k, v := range assets.AssetCache {
		response.Items = append(response.Items, defs.CachedItem{Name: k, LastUsed: v.LastUsed, Count: v.Count, Class: defs.AssetCacheClass})
	}

	// Sort the results. By default, the array is sorted by the URL which is the path to the
	// cached objects. Other sort orders are supported, "count" and "time". The sort order
	// keyword names are case-insensitive.
	sortBy := "url"
	if session.Parameters["order-by"] != nil {
		sortBy = strings.ToLower(session.Parameters["order-by"][0])
	}

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
		sort.Slice(response.Items, func(i, j int) bool {
			return response.Items[i].Count > response.Items[j].Count
		})

	case "time", "age", "last-used":
		sort.Slice(response.Items, func(i, j int) bool {
			return response.Items[i].LastUsed.After(response.Items[j].LastUsed)
		})

	default:
		return util.ErrorResponse(w, session.ID, "invalid sort order: "+sortBy, http.StatusBadRequest)
	}

	w.Header().Add(defs.ContentTypeHeader, defs.CacheMediaType)
	b := util.WriteJSON(w, response, &session.ResponseLength)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}
