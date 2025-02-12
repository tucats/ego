// Package assets handles the server side asset caching of arbitrary
// objects. This is typically used to provide server-side caching of
// objects retrieved via the /assets/ endpoint. This is most often
// used in HTML pages accessing static information in the server.
package assets

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/util"
)

// AssetsHandler is the handler for the GET method on the assets endpoint. The handler
// must be passed a relative endpoint path from the "/assets" endpoint.
//
// The handler first ensures the path name is relative by removing any leading slash or
// dots. If the resulting path is in the cache, the cached value is returned to the
// caller. If not in cache, attempt to read the file at the designated path within the
// assets directory, add it to the cache, and return the result.
func AssetsHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var (
		err  error
		path = r.URL.Path
	)

	// We dont permit index requests
	if path == "" || strings.HasSuffix(path, "/") {
		ui.Log(ui.AssetLogger, "asset.index", ui.A{
			"session": session.ID,
			"path":    path})
		w.WriteHeader(http.StatusForbidden)

		msg := fmt.Sprintf(`{"err": "%s"}`, "index reads not permitted")
		_, _ = w.Write([]byte(msg))
		session.ResponseLength += len(msg)

		return http.StatusForbidden
	}

	// Get the asset data from the cache or load it from the file system as needed. If this
	// results in an error, return an error response.
	data, err := Loader(session.ID, path)
	if err != nil {
		root := ""
		if libpath := settings.Get(defs.EgoLibPathSetting); libpath != "" {
			root = libpath
		} else {
			root = filepath.Join(settings.Get(defs.EgoPathSetting), defs.LibPathName)
		}

		errorMsg := strings.ReplaceAll(err.Error(), filepath.Join(root, "services"), "")
		msg := fmt.Sprintf(`{"err": "%s"}`, errorMsg)

		ui.Log(ui.AssetLogger, "asset.load.error", ui.A{
			"session": session.ID,
			"path":    path,
			"error":   err.Error()})
		w.WriteHeader(http.StatusBadRequest)

		_, _ = w.Write([]byte(msg))
		session.ResponseLength += len(msg)

		return http.StatusBadRequest
	}

	// Are we being asked to return just a portion of the asset because there is a range
	// specification in the request?
	start := 0
	end := len(data)
	hasRange := ""

	if h, found := r.Header["Range"]; found && len(h) > 0 {
		// Parse the range header and extract the start and end byte positions.
		text := strings.ReplaceAll(h[0], "bytes=", "")
		ranges := strings.Split(text, "-")

		if len(ranges) > 0 {
			start, err = egostrings.Atoi(ranges[0])
			if err != nil {
				return util.ErrorResponse(w, session.ID, "Invalid range header: "+h[0], http.StatusBadRequest)
			}
		}

		if len(ranges) > 1 {
			end, err = egostrings.Atoi(ranges[1])
			if err != nil {
				return util.ErrorResponse(w, session.ID, "Invalid range header: "+h[0], http.StatusBadRequest)
			}

			hasRange = fmt.Sprintf(" range %d-%d;", start, end)
		}
	}

	// Determine the portion of the asset we are going to be returning. If there
	// was no range specified, start and end result in us returning the entire asset.
	slice := data[start:end]

	// Map the extension type of the asset into a content type value if possible.
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

	// If there is a range specified, set the Content-Range and Accept-Ranges headers to
	// show what part of the range we returned.
	if hasRange != "" {
		w.Header()["Content-Range"] = []string{fmt.Sprintf("bytes %d-%d/%d", start, end, len(data))}
		w.Header()["Accept-Ranges"] = []string{"bytes"}
	}

	// Write the status of the request and the actual asset to the response and we're done.
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(slice)
	session.ResponseLength += len(slice)

	return http.StatusOK
}

// Loader is the function that is called to load an asset from within the server. This
// function is called by the server to load an asset from the server's file system. The
// sessionID is used for logging purposes, and the path is the relative path to the asset
// to be loaded. This allows server handlers like the /ui endpoint to load assets from
// the server's file system.
//
// If the item can be found in the cache, it is returned. If not, the file is read from
// the file system, added to the cache, and returned to the caller.
func Loader(sessionID int, path string) ([]byte, error) {
	var err error

	data := lookupCachedAsset(sessionID, path)
	if data == nil {
		data, err = readAssetFile(sessionID, path)
		if err == nil {
			cacheAsset(sessionID, path, data)
		}
	}

	return data, err
}

// readAssetFile reads an asset file from the server's file system. The sessionID is used
// for logging purposes, and the path is the relative path to the asset to be loaded.
// The path is sanitized to remove any leading dots or slashes, and the file is read
// from the server's file system.
func readAssetFile(sessionID int, path string) ([]byte, error) {
	for strings.HasPrefix(path, ".") || strings.HasPrefix(path, "/") {
		path = path[1:]
	}

	// Remove any ".." notations from the file path
	path = strings.ReplaceAll(path, "..", "")

	// Graft the resulting path onto the root path for the assets.
	root := ""
	if libpath := settings.Get(defs.EgoLibPathSetting); libpath != "" {
		root = libpath
	} else {
		root = filepath.Join(settings.Get(defs.EgoPathSetting), defs.LibPathName)
	}

	// Build the final full path name, and for safety remove any ".." notations
	// left in the path.
	fn := filepath.Join(root, path)
	fn = strings.ReplaceAll(fn, "..", "")

	// Read the data from the resulting location.
	data, err := os.ReadFile(fn)

	if err == nil {
		if sessionID > 0 {
			ui.Log(ui.AssetLogger, "asset.read", ui.A{
				"session": sessionID,
				"size":    len(data),
				"path":    fn})
		} else {
			ui.Log(ui.AssetLogger, "asset.read.local", ui.A{
				"size": len(data),
				"path": fn})
		}
	} else {
		if sessionID > 0 {
			ui.Log(ui.AssetLogger, "asset.load.error", ui.A{
				"session": sessionID,
				"path":    fn,
				"error":   err})
		} else {
			ui.Log(ui.AssetLogger, "asset.load.local.error", ui.A{
				"path":  fn,
				"error": err})
		}
	}

	return os.ReadFile(fn)
}
