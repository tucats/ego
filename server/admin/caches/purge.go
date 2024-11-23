package caches

import (
	"net/http"

	"github.com/tucats/ego/server/assets"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/server/services"
)

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
