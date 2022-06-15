package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/http/assets"
	"github.com/tucats/ego/http/services"
	"github.com/tucats/ego/util"
)

// FlushCacheHandler is the rest handler for /admin/caches endpoint.
func cachesAction(sessionID int32, w http.ResponseWriter, r *http.Request) int {
	user, hasAdminPrivileges := isAdminRequestor(r)
	if !hasAdminPrivileges {
		ui.Debug(ui.AuthLogger, "[%d] User %s not authorized", sessionID, user)
		util.ErrorResponse(w, sessionID, "Not authorized", http.StatusForbidden)

		return http.StatusForbidden
	}

	logHeaders(r, sessionID)

	switch r.Method {
	case http.MethodPost:
		var result defs.CacheResponse

		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)

		err := json.Unmarshal(buf.Bytes(), &result)
		if errors.Nil(err) {
			services.MaxCachedEntries = result.Limit
		}

		if !errors.Nil(err) {
			util.ErrorResponse(w, sessionID, err.Error(), http.StatusBadRequest)

			return http.StatusBadRequest
		} else {
			result = defs.CacheResponse{
				ServerInfo: util.MakeServerInfo(sessionID),
				Count:      len(services.ServiceCache),
				Limit:      services.MaxCachedEntries,
				Items:      []defs.CachedItem{},
			}

			for k, v := range services.ServiceCache {
				result.Items = append(result.Items, defs.CachedItem{Name: k, LastUsed: v.Age})
			}
		}

		w.Header().Add(contentTypeHeader, defs.CacheMediaType)

		b, _ := json.Marshal(result)
		_, _ = w.Write(b)

		ui.Debug(ui.ServerLogger, fmt.Sprintf("[%d] %d, sending JSON response", sessionID, http.StatusOK))

		return http.StatusOK

	// Get the list of cached items.
	case http.MethodGet:
		result := defs.CacheResponse{
			ServerInfo: util.MakeServerInfo(sessionID),
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

		w.Header().Add(contentTypeHeader, defs.CacheMediaType)

		b, _ := json.Marshal(result)
		_, _ = w.Write(b)

		ui.Debug(ui.ServerLogger, "[%d] 200, sending JSON response", sessionID)

		return http.StatusOK

	// DELETE the cached service compilation units. In-flight services
	// are unaffected.
	case http.MethodDelete:
		assets.FlushAssetCache()

		services.ServiceCache = map[string]services.CachedCompilationUnit{}
		result := defs.CacheResponse{
			ServerInfo: util.MakeServerInfo(sessionID),
			Count:      0,
			Limit:      services.MaxCachedEntries,
			Items:      []defs.CachedItem{},
			AssetSize:  assets.GetAssetCacheSize(),
			AssetCount: assets.GetAssetCacheCount(),
		}

		w.Header().Add(contentTypeHeader, defs.CacheMediaType)

		b, _ := json.Marshal(result)
		_, _ = w.Write(b)

		ui.Debug(ui.ServerLogger, "[%d] 200, sending JSON response", sessionID)

		return http.StatusOK

	default:
		util.ErrorResponse(w, sessionID, "Unsupported method: "+r.Method, http.StatusTeapot)

		return http.StatusTeapot
	}
}
