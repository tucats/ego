package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
)

type asset struct {
	data     []byte
	count    int
	lastUsed time.Time
}

var assetCache map[string]asset
var assetMux sync.Mutex
var maxAssetCacheSize int = 1024 * 1024 // How many bytes can we keep in the cache? Default is 1MB.
var assetCacheSize int = 0

// Flush the cache of assets being held in memory on behalf of the html services.
func FlushAssetCache() {
	assetMux.Lock()
	defer assetMux.Unlock()

	assetCache = map[string]asset{}
	assetCacheSize = 0

	ui.Debug(ui.ServerLogger, "Initialized asset cache; max size %d", maxAssetCacheSize)
}

// Get the current asset cache size.
func GetAssetCacheSize() int {
	return assetCacheSize
}

// Get the number of items in the asset cache.
func GetAssetCacheCount() int {
	assetMux.Lock()
	defer assetMux.Unlock()

	return len(assetCache)
}

// For a given asset path, look it up in the cache. If found, the asset is returned
// as a byte array. If not found, a nil value is returned.
func findAsset(sessionID int32, path string) []byte {
	assetMux.Lock()
	defer assetMux.Unlock()

	if assetCache == nil {
		assetCache = map[string]asset{}

		ui.Debug(ui.ServerLogger, "[%d] Initialized asset cache, %d bytes", sessionID, maxAssetCacheSize)
	}

	if a, ok := assetCache[path]; ok {
		a.lastUsed = time.Now()
		a.count = a.count + 1
		assetCache[path] = a

		ui.Debug(ui.InfoLogger, "[%d] Asset loaded from cache: %s", sessionID, path)

		return a.data
	}

	ui.Debug(ui.InfoLogger, "[%d] Asset not found in cache: %s", sessionID, path)

	return nil
}

// For a given asset path and an asset byte array, store it in the cache. If the cache
// grows too large, then drop objects from the cache, oldest-first.
func saveAsset(sessionID int32, path string, data []byte) {
	if len(data) > maxAssetCacheSize/2 {
		ui.Debug(ui.InfoLogger, "[%d] Asset too large to cache; path %s; size %d; cache size %d",
			sessionID, path, len(data), assetCacheSize)

		return
	}

	assetMux.Lock()
	defer assetMux.Unlock()

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	a := asset{
		data:     data,
		lastUsed: time.Now(),
	}

	// Does it already exist? If so, delete the old size
	if oldAsset, found := assetCache[path]; found {
		assetCacheSize = assetCacheSize - len(oldAsset.data)
	}

	// Add in the new asset, and increment the asset cache size
	assetCache[path] = a
	newSize := len(a.data)
	assetCacheSize = assetCacheSize + newSize

	// If the cache is too big, delete stuff until it shrinks enough
	for assetCacheSize > maxAssetCacheSize {
		oldestAsset := ""
		oldestTime := time.Now()

		for k, v := range assetCache {
			age := v.lastUsed.Sub(oldestTime)
			if age < 0 {
				oldestTime = v.lastUsed
				oldestAsset = k
			}
		}

		oldSize := len(assetCache[oldestAsset].data)
		assetCacheSize = assetCacheSize - oldSize

		delete(assetCache, oldestAsset)

		ui.Debug(ui.InfoLogger, "[%d] Asset purged; path %s; size %d; cache size %d",
			sessionID, path, oldSize, assetCacheSize)
	}

	ui.Debug(ui.InfoLogger, "[%d] Asset saved; path %s; size %d; cache size %d",
		sessionID, path, newSize, assetCacheSize)
}

// Registered handler for the /assets path. Ensure the path name is relative by removing
// any leading slash or dots. If the resulting path is in the cache, the cached value is
// returned to the caller. If not in cache, attempt to read the file at the designated
// path within the assets directory, add it to the cache, and return the result.
func AssetsHandler(w http.ResponseWriter, r *http.Request) {
	var err error

	CountRequest(AssetRequestCounter)

	sessionID := atomic.AddInt32(&nextSessionID, 1)
	path := r.URL.Path

	ui.Debug(ui.RestLogger, "[%d] User agent: %s", sessionID, r.Header.Get("User-Agent"))

	// We dont permit index requests
	if path == "" || strings.HasSuffix(path, "/") {
		w.WriteHeader(http.StatusForbidden)

		msg := fmt.Sprintf(`{"err": "%s"}`, "index reads not permitted")
		_, _ = w.Write([]byte(msg))

		ui.Debug(ui.InfoLogger, "[%d] Indexed asset read attempt from path %s", sessionID, path)
		ui.Debug(ui.InfoLogger, "[%d] STATUS 403, sending JSON response", sessionID)

		return
	}

	data := findAsset(sessionID, path)
	if data == nil {
		for strings.HasPrefix(path, ".") || strings.HasPrefix(path, "/") {
			path = path[1:]
		}

		root := settings.Get("ego.runtime.path")
		fn := filepath.Join(root, "lib/services", path)

		ui.Debug(ui.InfoLogger, "[%d] Asset read from file %s", sessionID, fn)

		data, err = ioutil.ReadFile(fn)
		if err != nil {
			errorMsg := strings.ReplaceAll(err.Error(), filepath.Join(root, "lib/services"), "")

			msg := fmt.Sprintf(`{"err": "%s"}`, errorMsg)

			ui.Debug(ui.InfoLogger, "[%d] Server asset load error: %s", sessionID, err.Error())
			w.WriteHeader(400)
			_, _ = w.Write([]byte(msg))

			return
		}

		saveAsset(sessionID, path, data)
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
