package caches

import (
	"net/http"
	"strings"

	"github.com/tucats/ego/caches"
	"github.com/tucats/ego/server/assets"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/server/services"
)

// PurgeCacheHandler is the cache endpoint handler that purges all entries in the cache,
// and then returns the (revised) cache status.
func PurgeCacheHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	// See if we are purging all entries or a specific set of caches.
	if len(session.Parameters["class"]) == 0 {
		// Free up the various caches used to support authentication and DSN handling.
		caches.PurgeAll()

		// Release the entries in the user cache.
		// Release the entries in the asset cache.
		assets.FlushAssetCache()

		// Release the entries in the service cache.
		services.FlushServiceCache()
	} else {
		for _, class := range session.Parameters["class"] {
			switch strings.ToLower(class) {
			case "authorizations", "authorization", "permission", "permissions":
				caches.Purge(caches.AuthCache)

			case "user", "users", "authentication":

				caches.Purge(caches.UserCache)

			case "dsn", "dsns":
				caches.Purge(caches.DSNCache)

			case "token", "tokens":
				caches.Purge(caches.TokenCache)

			case "blacklist":
				caches.Purge(caches.BlacklistCache)

			case "schema", "schemas":
				caches.Purge(caches.SchemaCache)

			case "service", "services":
				services.FlushServiceCache()

			case "asset", "assets":
				assets.FlushAssetCache()
			}
		}
	}

	// Return the (revised) cache status
	return GetCacheHandler(session, w, r)
}
