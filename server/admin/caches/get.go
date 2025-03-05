package caches

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/assets"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/server/services"
	"github.com/tucats/ego/util"
)

// GetCacheHandler is the cache endpoint handler for retrieving the cache status from the server.
func GetCacheHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	result := defs.CacheResponse{
		ServerInfo:        util.MakeServerInfo(session.ID),
		ServiceCount:      len(services.ServiceCache),
		ServiceCountLimit: services.MaxCachedEntries,
		Items:             []defs.CachedItem{},
		AssetSize:         assets.GetAssetCacheSize(),
		AssetCount:        assets.GetAssetCacheCount(),
		Status:            http.StatusOK,
	}

	for k, v := range services.ServiceCache {
		result.Items = append(result.Items, defs.CachedItem{Name: k, LastUsed: v.Age, Count: v.Count, Class: defs.ServiceCacheClass})
	}

	for k, v := range assets.AssetCache {
		result.Items = append(result.Items, defs.CachedItem{Name: k, LastUsed: v.LastUsed, Count: v.Count, Class: defs.AssetCacheClass})
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
		sort.Slice(result.Items, func(i, j int) bool {
			return result.Items[i].Class < result.Items[j].Class
		})
	case "url", "name", "path":
		sort.Slice(result.Items, func(i, j int) bool {
			return result.Items[i].Name < result.Items[j].Name
		})

	case "count", "hits":
		sort.Slice(result.Items, func(i, j int) bool {
			return result.Items[i].Count > result.Items[j].Count
		})

	case "time", "age", "last-used":
		sort.Slice(result.Items, func(i, j int) bool {
			return result.Items[i].LastUsed.After(result.Items[j].LastUsed)
		})

	default:
		return util.ErrorResponse(w, session.ID, "invalid sort order: "+sortBy, http.StatusBadRequest)
	}

	w.Header().Add(defs.ContentTypeHeader, defs.CacheMediaType)

	b, _ := json.MarshalIndent(result, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	_, _ = w.Write(b)
	session.ResponseLength += len(b)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}
