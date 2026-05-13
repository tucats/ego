package caches

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/router"
	"github.com/tucats/ego/server/services"
	"github.com/tucats/ego/util"
)

// SetCacheSizeHandler is the HTTP handler for POST /admin/caches. It reads a
// JSON body containing a ServiceCountLimit value and applies it as the new
// maximum number of compiled Ego services to keep in the service cache. It
// then returns the revised cache status by delegating to GetCacheHandler.
func SetCacheSizeHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	var result defs.CacheResponse

	// bytes.Buffer is an in-memory byte buffer. We read the entire request body
	// into it so we can both decode it as JSON and later log its raw text.
	// The blank identifier _ discards the byte count and error from ReadFrom,
	// which are not needed here since json.Unmarshal reports any read errors.
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r.Body)

	// json.Unmarshal decodes the JSON bytes into the result struct. Only the
	// fields present in the JSON are updated; the rest keep their zero values.
	err := json.Unmarshal(buf.Bytes(), &result)
	if err == nil {
		// Apply the new service-cache size limit from the request body.
		services.MaxCachedEntries = result.ServiceCountLimit
	} else {
		// The body was not valid JSON — log the error and return 400 Bad Request.
		ui.Log(ui.RestLogger, "rest.bad.payload", ui.A{
			"session": session.ID,
			"error":   err})

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	// Log the raw request body now that we know it was valid.
	ui.Log(ui.RestLogger, "rest.request.payload", ui.A{
		"session": session.ID,
		"body":    buf.String()})

	// Return the (revised) cache status so the client can confirm the new limit
	// is in effect without needing a separate GET request.
	return GetCacheHandler(session, w, r)
}
