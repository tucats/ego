// Package assets handles the server side asset caching of arbitrary
// objects. This is typically used to provide server-side caching of
// objects retrieved via the /assets/ endpoint. This is most often
// used in HTML pages accessing static information in the server.
package assets

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/javascript"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/util"
)

const (
	StartOfData  = int64(0)
	EndOfData    = int64(math.MaxInt64)
	MaxAssetSize = 2 * 1024 * 1024 // 2MB, largest item we'll read using a range
)

// Set this to true when you want the asset loader to detect range operations and
// be smart about how to load them from disk in fragments. If this is turned off,
// range operations will attempt to use the cache, and if not found in cache, will
// re-read the entire asset into memory before determining the range. This can be a
// big memory consumer for huge assets (like video, for example). Set this to false
// if debugging a problem with range handling.
var smartRangeLoading = true

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

	// We don't permit index requests
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

	// Do not permit relative path specifications to avoid poking _above_
	// the asset directory tree.
	if strings.Contains(path, "/../") {
		ui.Log(ui.AssetLogger, "asset.relative", ui.A{
			"session": session.ID,
			"path":    path})
		w.WriteHeader(http.StatusForbidden)

		msg := fmt.Sprintf(`{"err": "%s"}`, "relative path reads not permitted")
		_, _ = w.Write([]byte(msg))
		session.ResponseLength += len(msg)

		return http.StatusForbidden
	}

	// Are we being asked to return just a portion of the asset because there is a range
	// specification in the request?
	start := StartOfData
	end := EndOfData

	hasRange := ""

	if h, found := r.Header["Range"]; found && len(h) > 0 {
		// Parse the range header and extract the start and end byte positions.
		text := strings.ReplaceAll(h[0], "bytes=", "")
		ranges := strings.Split(text, "-")

		if len(ranges) > 0 {
			start, err = strconv.ParseInt(ranges[0], 10, 64)
			if err != nil {
				return util.ErrorResponse(w, session.ID, "Invalid range header: "+h[0], http.StatusBadRequest)
			}
		}

		if len(ranges) > 1 && ranges[1] != "" {
			end, err = strconv.ParseInt(ranges[1], 10, 64)
			if err != nil {
				return util.ErrorResponse(w, session.ID, "Invalid range header: "+h[0], http.StatusBadRequest)
			}

			hasRange = fmt.Sprintf(" range %d-%d;", start, end)
		} else if ranges[1] == "" {
			// Open-ended range (e.g. "bytes=500-"): end resolved after file load.
			hasRange = fmt.Sprintf(" range %d-;", start)
		}
	}

	// Sanity check on explicit ranges only.
	if start < 0 || (end != EndOfData && end < start) {
		return util.ErrorResponse(w, session.ID, errors.ErrInvalidRange.Context(hasRange).Error(), http.StatusBadRequest)
	}

	// Always load the full asset so we know the total file size. Range slicing
	// is applied below after we capture totalSize. This also lets the cache work
	// normally — range reads still benefit from a previously cached full asset.
	data, totalSize, err := Loader(session.ID, path, start, end)
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
		w.WriteHeader(http.StatusNotFound)

		_, _ = w.Write([]byte(msg))
		session.ResponseLength += len(msg)

		return http.StatusNotFound
	}

	// Is the asset an .md (markdown) file? If so render it as HTML.
	if strings.HasSuffix(path, ".md") {
		data = mdToHTML(data)
	}

	// Map the extension type of the asset into a content type value if possible.
	ext := filepath.Ext(path)
	if t, found := map[string]string{
		".md":   "text/html",
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

	// Always advertise range support so browsers know they can request partial content.
	w.Header()["Accept-Ranges"] = []string{"bytes"}

	// Apply range slicing if requested. HTTP Range end byte is inclusive (bytes=0-1 → 2 bytes),
	// so slice as data[start:end+1]. Clamp end to actual file size for open-ended ranges.
	status := http.StatusOK

	if hasRange != "" {
		w.Header()["Content-Range"] = []string{fmt.Sprintf("bytes %d-%d/%d", start, end, totalSize)}
		w.Header()["Content-Length"] = []string{strconv.FormatInt(int64(len(data)), 10)}
		status = http.StatusPartialContent
	}

	// Write the status of the request and the actual asset to the response and we're done.
	w.WriteHeader(status)
	_, _ = w.Write(data)
	session.ResponseLength += len(data)

	return status
}

// Loader is the function that is called to load an asset from within the server. This
// function is called by the server to load an asset from the server's file system. The
// sessionID is used for logging purposes, and the path is the relative path to the asset
// to be loaded. This allows server handlers like the /ui endpoint to load assets from
// the server's file system.
//
// If the range is default, try to find the item in the cache. If found, it is returned.
// If not, the file is read from the file system, added to the cache, and returned to the
// caller.
//
// By default, if a range is specified, an alternate load is used to read the range
// directly from the filesystem. Even if range loading is disabled, range reads are
// not cached.
//
// The results are the actual data read (the ranged slice, or the entire asset), the
// total size of the item (irrespective of ranging) and an error code.
func Loader(sessionID int, path string, start, end int64) ([]byte, int64, error) {
	var (
		data      []byte
		err       error
		totalSize int64
	)

	if start < 0 || end < 0 {
		return nil, 0, errors.ErrInvalidRange.Context(fmt.Sprintf("%d-%d", start, end))
	}

	// If we're not using a range specification, see if the object is in the
	// cache, and if not load from disk and save it in the cache.
	if !smartRangeLoading || (start == StartOfData && end == EndOfData) {
		data = lookupCachedAsset(sessionID, path)
		if data == nil {
			data, err = readAssetFile(sessionID, path)
			if err == nil {
				if strings.HasSuffix(path, ".js") && settings.GetBool(defs.JSMinifySetting) {
					original := len(data)
					data = javascript.Minify(data, settings.GetBool(defs.JSShortVarNamesSetting))
					minified := len(data)
					saved := original - minified
					pct := 0

					if original > 0 {
						pct = saved * 100 / original
					}

					ui.Log(ui.AssetLogger, "asset.minify", ui.A{
						"session":  sessionID,
						"path":     path,
						"original": original,
						"size":     minified,
						"saved":    saved,
						"pct":      pct,
					})
				}

				totalSize = int64(len(data))

				// If we got a range, trim the data now. This would only happen if "smartRangeLoad"
				// had been turned off (typically done for debugging only). If we have a default
				// range, store the data item in the cache for optimizing future reads.
				if start != StartOfData || end != EndOfData {
					data = data[start:end]
				} else {
					cacheAsset(sessionID, path, data)
				}
			}
		}
	} else {
		// We have a range specification, so do a load seeking to the desired place.
		// When a range is specified, the cache is never used.
		data, totalSize, err = readAssetRange(sessionID, path, start, end)
	}

	return data, totalSize, err
}

// readAssetRange is the function that is called to load an asset with a range. The
// sessionID is used for logging purposes, and the path is the relative path to the asset
// to be loaded. This version is only called when the loader detects that a range header
// is present in the HTTP request.
func readAssetRange(sessionID int, path string, start, end int64) ([]byte, int64, error) {
	var (
		data      []byte
		totalSize int64
	)

	// Make the path into a valid path localized to where the assets live in file system.
	fn := normalizeAssetPath(path)

	// We need to know the total size of the file contents for accurate range reporting,
	// even though we expect to only read a part of it.
	info, err := os.Stat(fn)
	if err != nil {
		return nil, totalSize, errors.New(err)
	}

	totalSize = info.Size()

	// Range reads mean we need to access the file handle.
	file, err := os.Open(fn)
	if err != nil {
		return nil, totalSize, errors.New(err)
	}

	defer file.Close()

	// Read bytes into a buffer the size of the requested range
	size := end - start
	data = make([]byte, size)

	// Read starting at the given location, filling the buffer for as
	// many bytes as asked to load. If we get an EOF, that's okay; the
	// count will say how bit the data really is.
	count, err := file.ReadAt(data, start)
	if err != nil && err != io.EOF {
		return nil, totalSize, errors.New(err)
	}

	// This truncates the last short read when there's an EOF. Otherwise,
	// it does nothing.
	data = data[:count]

	ui.Log(ui.AssetLogger, "asset.read.range", ui.A{
		"session": sessionID,
		"path":    path,
		"start":   start,
		"end":     end,
	})

	return data, totalSize, err
}

// readAssetFile reads an asset file from the server's file system. The sessionID is used
// for logging purposes, and the path is the relative path to the asset to be loaded.
// The path is sanitized to remove any leading dots or slashes, and the file is read
// from the server's file system.
func readAssetFile(sessionID int, path string) ([]byte, error) {
	// Make the path into a valid path localized to where the assets live in file system.
	fn := normalizeAssetPath(path)

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

func normalizeAssetPath(path string) string {
	//  Strip off any dots or slashes at the start of the path.
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

	return fn
}
