package caches

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/server/services"
	"github.com/tucats/ego/util"
)

// SetCacheSizeHandler is the cache endpoint handler for setting caache size,
// using the cache specification value found in the request body. The request
// returns the (revised) cache status to the calling client.
func SetCacheSizeHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var result defs.CacheResponse

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r.Body)

	err := json.Unmarshal(buf.Bytes(), &result)
	if err == nil {
		services.MaxCachedEntries = result.ServiceCountLimit
	} else {
		ui.Log(ui.RestLogger, "rest.bad.payload",
			"session", session.ID,
			"error", err)

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	// Return the (revised) cache status
	return GetCacheHandler(session, w, r)
}
