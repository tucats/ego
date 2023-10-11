package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/assets"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/server/services"
	"github.com/tucats/ego/util"
)

// SetCacheSizeHandler is the cache endpoint handler for setting caache size, using the cache
// specification value found in the request body. The request returns the (revised) cache status.
func SetCacheSizeHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var result defs.CacheResponse

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r.Body)

	err := json.Unmarshal(buf.Bytes(), &result)
	if err == nil {
		services.MaxCachedEntries = result.Limit
	} else {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	// Return the (revised) cache status
	return GetCacheHandler(session, w, r)
}

// PurgeCacheHandler is the cache endpoint handler that purges all entries in the cache,
// and then returns the (revised) cache status.
func PurgeCacheHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	// Release the entries in the asset cache.
	assets.FlushAssetCache()

	// Release the entries in the service cache.
	services.FlushServiceCache()

	// Return the (revised) cache status
	return GetCacheHandler(session, w, r)
}

// GetCacheHandler is the cache endpoint handler for retrieving the cache status from the server.
func GetCacheHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	result := defs.CacheResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Count:      len(services.ServiceCache) + len(assets.AssetCache),
		Limit:      services.MaxCachedEntries,
		Items:      []defs.CachedItem{},
		AssetSize:  assets.GetAssetCacheSize(),
		AssetCount: assets.GetAssetCacheCount(),
		Status:     http.StatusOK,
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

	b, _ := json.Marshal(result)
	_, _ = w.Write(b)

	return http.StatusOK
}
