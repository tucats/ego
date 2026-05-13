package caches

import (
	"net/http"
	"strings"

	"github.com/tucats/ego/caches"
	"github.com/tucats/ego/router"
	"github.com/tucats/ego/server/assets"
	"github.com/tucats/ego/server/services"
)

// PurgeCacheHandler is the HTTP handler for DELETE /admin/caches. It discards
// all (or a named subset of) in-memory cache entries and then returns the
// revised cache status by delegating to GetCacheHandler.
//
// The optional "class" query parameter may be repeated to target one or more
// specific caches (e.g. ?class=tokens&class=dsns). Omitting it purges every
// cache at once.
func PurgeCacheHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	// session.Parameters["class"] is a []string of all values provided for the
	// "class" query parameter. len() == 0 means the parameter was not supplied.
	if len(session.Parameters["class"]) == 0 {
		// No class filter — purge everything.  Free up the various caches
		// used to support authentication and DSN handling.
		caches.PurgeAll()

		// Release the entries in the asset cache (static files).
		assets.FlushAssetCache()

		// Release the entries in the service cache (compiled Ego programs).
		services.FlushServiceCache()
	} else {
		// One or more class names were supplied. Loop over them and purge each
		// named cache. strings.ToLower makes the comparison case-insensitive so
		// "Tokens", "TOKENS", and "tokens" all match.
		for _, class := range session.Parameters["class"] {
			switch strings.ToLower(class) {
			case "authorizations", "authorization", "permission", "permissions":
				caches.Purge(caches.AuthCache)

			case "user", "users":
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

	// Return the (revised) cache status so the client can see the result of
	// the purge without needing a separate GET request.
	return GetCacheHandler(session, w, r)
}
