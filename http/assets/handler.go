// Package assets handles the server side asset caching of arbitrary
// objects. This is typically used to provide server-side caching of
// objects retrieved via the /assets/ endpoint. This is most often
// used in HTML pages accessing static information in the server.
package assets

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	server "github.com/tucats/ego/http/server"
)

// AssetsHandler is the handler for the GET method on the assets endpoint. The handler
// must be passed a relative endpoint path from the "/assets" endpoint.
//
// The handler first ensures the path name is relative by removing any leading slash or
// dots. If the resulting path is in the cache, the cached value is returned to the
// caller. If not in cache, attempt to read the file at the designated path within the
// assets directory, add it to the cache, and return the result.
func AssetsHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var err error

	path := r.URL.Path

	// We dont permit index requests
	if path == "" || strings.HasSuffix(path, "/") {
		ui.Log(ui.RestLogger, "[%d] Indexed asset read attempt from path %s", session.ID, path)
		w.WriteHeader(http.StatusForbidden)

		msg := fmt.Sprintf(`{"err": "%s"}`, "index reads not permitted")
		_, _ = w.Write([]byte(msg))

		return http.StatusForbidden
	}

	data := findAsset(session.ID, path)
	if data == nil {
		for strings.HasPrefix(path, ".") || strings.HasPrefix(path, "/") {
			path = path[1:]
		}

		root := ""
		if libpath := settings.Get(defs.EgoLibPathSetting); libpath != "" {
			root = libpath
		} else {
			root = filepath.Join(settings.Get(defs.EgoPathSetting), defs.LibPathName)
		}

		fn := filepath.Join(root, "services", path)

		ui.Log(ui.RestLogger, "[%d] Asset read from file %s", session.ID, fn)

		data, err = ioutil.ReadFile(fn)
		if err != nil {
			errorMsg := strings.ReplaceAll(err.Error(), filepath.Join(root, "services"), "")
			msg := fmt.Sprintf(`{"err": "%s"}`, errorMsg)

			ui.Log(ui.RestLogger, "[%d] Server asset load error: %s", session.ID, err.Error())
			w.WriteHeader(http.StatusBadRequest)

			_, _ = w.Write([]byte(msg))

			return http.StatusBadRequest
		}

		saveAsset(session.ID, path, data)
	}

	start := 0
	end := len(data)
	hasRange := ""

	if h, found := r.Header["Range"]; found && len(h) > 0 {
		text := strings.ReplaceAll(h[0], "bytes=", "")

		ranges := strings.Split(text, "-")
		if len(ranges) > 0 {
			start, _ = strconv.Atoi(ranges[0])
		}

		if len(ranges) > 1 {
			end, _ = strconv.Atoi(ranges[1])
			hasRange = fmt.Sprintf(" range %d-%d;", start, end)
		}
	}

	slice := data[start:end]

	// Map the extension type of the object into a content type
	// value if possible.
	ext := filepath.Ext(path)
	if t, found := map[string]string{
		".txt":  "application/text",
		".text": "application/text",
		".json": "application/json",
		".mp4":  "video/mp4",
		".pdf":  "application/pdf",
		".htm":  "text/html",
		".html": "text/html",
		".css":  "text/css",
		".js":   "text/javascript",
	}[ext]; found {
		w.Header()["Content-Type"] = []string{t}
	}

	if hasRange != "" {
		w.Header()["Content-Range"] = []string{fmt.Sprintf("bytes %d-%d/%d", start, end, len(data))}
		w.Header()["Accept-Ranges"] = []string{"bytes"}
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(slice)

	return http.StatusOK
}
