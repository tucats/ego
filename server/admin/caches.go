package admin

import (
	"bytes"
	"encoding/json"
	"net/http"

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
	assets.FlushAssetCache()

	// Return the (revised) cache status
	return GetCacheHandler(session, w, r)
}

// GetCacheHandler is the cache endpoint handler for retrieving the cache status from the server.
func GetCacheHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	result := defs.CacheResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Count:      len(services.ServiceCache),
		Limit:      services.MaxCachedEntries,
		Items:      []defs.CachedItem{},
		AssetSize:  assets.GetAssetCacheSize(),
		AssetCount: assets.GetAssetCacheCount(),
	}

	for k, v := range services.ServiceCache {
		result.Items = append(result.Items, defs.CachedItem{Name: k, LastUsed: v.Age, Count: v.Count})
	}

	for k, v := range assets.AssetCache {
		result.Items = append(result.Items, defs.CachedItem{Name: k, LastUsed: v.LastUsed, Count: v.Count})
	}

	w.Header().Add(defs.ContentTypeHeader, defs.CacheMediaType)

	b, _ := json.Marshal(result)
	_, _ = w.Write(b)

	return http.StatusOK
}
