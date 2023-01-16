package assets

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	server "github.com/tucats/ego/http/server"
)

// Registered handler for the /assets path. Ensure the path name is relative by removing
// any leading slash or dots. If the resulting path is in the cache, the cached value is
// returned to the caller. If not in cache, attempt to read the file at the designated
// path within the assets directory, add it to the cache, and return the result.
func AssetsHandler(w http.ResponseWriter, r *http.Request) {
	var err error

	server.CountRequest(server.AssetRequestCounter)

	sessionID := atomic.AddInt32(&server.NextSessionID, 1)
	path := r.URL.Path

	server.LogRequest(r, sessionID)
	ui.Log(ui.RestLogger, "[%d] User agent: %s", sessionID, r.Header.Get("User-Agent"))

	// We dont permit index requests
	if path == "" || strings.HasSuffix(path, "/") {
		w.WriteHeader(http.StatusForbidden)

		msg := fmt.Sprintf(`{"err": "%s"}`, "index reads not permitted")
		_, _ = w.Write([]byte(msg))

		ui.Log(ui.InfoLogger, "[%d] Indexed asset read attempt from path %s", sessionID, path)
		ui.Log(ui.InfoLogger, "[%d] STATUS 403, sending JSON response", sessionID)

		return
	}

	data := findAsset(sessionID, path)
	if data == nil {
		for strings.HasPrefix(path, ".") || strings.HasPrefix(path, "/") {
			path = path[1:]
		}

		root := settings.Get(defs.EgoPathSetting)
		fn := filepath.Join(root, defs.LibPathName, "services", path)

		ui.Log(ui.InfoLogger, "[%d] Asset read from file %s", sessionID, fn)

		data, err = ioutil.ReadFile(fn)
		if err != nil {
			errorMsg := strings.ReplaceAll(err.Error(), filepath.Join(root, defs.LibPathName, "services"), "")

			msg := fmt.Sprintf(`{"err": "%s"}`, errorMsg)

			ui.Log(ui.InfoLogger, "[%d] Server asset load error: %s", sessionID, err.Error())
			w.WriteHeader(400)
			_, _ = w.Write([]byte(msg))

			return
		}

		saveAsset(sessionID, path, data)
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
