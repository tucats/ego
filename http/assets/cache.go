package assets

import (
	"strings"
	"sync"
	"time"

	"github.com/tucats/ego/app-cli/ui"
)

type assetObject struct {
	data     []byte
	Count    int
	LastUsed time.Time
}

// AssetCache is the map that identifies the objects that are in the
// cache. Each asset has a unique name (typially the endpoint path used
// to reference it in HTML code).
var AssetCache map[string]assetObject
var assetMux sync.Mutex
var maxAssetCacheSize int = 1024 * 1024 // How many bytes can we keep in the cache? Default is 1MB.
var assetCacheSize int = 0

// Flush the cache of assets being held in memory on behalf of the html services.
func FlushAssetCache() {
	assetMux.Lock()
	defer assetMux.Unlock()

	AssetCache = map[string]assetObject{}
	assetCacheSize = 0

	ui.Log(ui.ServerLogger, "Initialized asset cache; max size %d", maxAssetCacheSize)
}

// Get the current asset cache size.
func GetAssetCacheSize() int {
	return assetCacheSize
}

// Get the number of items in the asset cache.
func GetAssetCacheCount() int {
	assetMux.Lock()
	defer assetMux.Unlock()

	return len(AssetCache)
}

// For a given asset path, look it up in the cache. If found, the asset is returned
// as a byte array. If not found, a nil value is returned.
func findAsset(sessionID int32, path string) []byte {
	assetMux.Lock()
	defer assetMux.Unlock()

	if AssetCache == nil {
		AssetCache = map[string]assetObject{}

		ui.Log(ui.ServerLogger, "[%d] Initialized asset cache, %d bytes", sessionID, maxAssetCacheSize)
	}

	if a, ok := AssetCache[path]; ok {
		a.LastUsed = time.Now()
		a.Count = a.Count + 1
		AssetCache[path] = a

		ui.Log(ui.InfoLogger, "[%d] Asset loaded from cache: %s", sessionID, path)

		return a.data
	}

	ui.Log(ui.InfoLogger, "[%d] Asset not found in cache: %s", sessionID, path)

	return nil
}

// For a given asset path and an asset byte array, store it in the cache. If the cache
// grows too large, then drop objects from the cache, oldest-first.
func saveAsset(sessionID int32, path string, data []byte) {
	if len(data) > maxAssetCacheSize/2 {
		ui.Log(ui.InfoLogger, "[%d] Asset too large to cache; path %s; size %d; cache size %d",
			sessionID, path, len(data), assetCacheSize)

		return
	}

	assetMux.Lock()
	defer assetMux.Unlock()

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	a := assetObject{
		data:     data,
		LastUsed: time.Now(),
	}

	// Does it already exist? If so, delete the old size
	if oldAsset, found := AssetCache[path]; found {
		assetCacheSize = assetCacheSize - len(oldAsset.data)
	}

	// Add in the new asset, and increment the asset cache size
	AssetCache[path] = a
	newSize := len(a.data)
	assetCacheSize = assetCacheSize + newSize

	// If the cache is too big, delete stuff until it shrinks enough
	for assetCacheSize > maxAssetCacheSize {
		oldestAsset := ""
		oldestTime := time.Now()

		for k, v := range AssetCache {
			age := v.LastUsed.Sub(oldestTime)
			if age < 0 {
				oldestTime = v.LastUsed
				oldestAsset = k
			}
		}

		oldSize := len(AssetCache[oldestAsset].data)
		assetCacheSize = assetCacheSize - oldSize

		delete(AssetCache, oldestAsset)

		ui.Log(ui.InfoLogger, "[%d] Asset purged; path %s; size %d; cache size %d",
			sessionID, path, oldSize, assetCacheSize)
	}

	ui.Log(ui.InfoLogger, "[%d] Asset saved; path %s; size %d; cache size %d",
		sessionID, path, newSize, assetCacheSize)
}
