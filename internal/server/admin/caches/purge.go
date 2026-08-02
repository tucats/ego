package caches

import (
	"net/http"
	"strings"

	"github.com/tucats/ego/internal/caches"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/server/assets"
	"github.com/tucats/ego/internal/server/services"
)

// PurgeCacheHandler is the HTTP handler for DELETE /admin/caches. It discards
// all (or a named subset of) in-memory cache entries and then returns the
// revised cache status by delegating to GetCacheHandler.
//
// The optional "class" query parameter may be repeated to target one or more
// specific caches (e.g. ?class=tokens&class=dsns). Omitting it purges every
// cache at once.
// namedClasses returns the cache class names actually supplied, discarding
// values that name nothing. A parameter that is present but empty is treated
// exactly like one that was never supplied.
func namedClasses(values []string) []string {
	classes := make([]string, 0, len(values))

	for _, value := range values {
		for _, class := range strings.Split(value, ",") {
			if class = strings.TrimSpace(class); class != "" {
				classes = append(classes, class)
			}
		}
	}

	return classes
}

func PurgeCacheHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	// session.Parameters["class"] is a []string of all values provided for the
	// "class" query parameter. An empty result means the parameter was either
	// not supplied at all, or supplied with no value (?class=) -- the "list"
	// parameter type accepts the latter, and an empty list means the same thing
	// as an absent one: no class filter, so purge everything.
	classes := namedClasses(session.Parameters["class"])

	if len(classes) == 0 {
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
		for _, class := range classes {
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
