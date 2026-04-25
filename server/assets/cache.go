package assets

import (
	"strings"
	"sync"
	"time"

	"github.com/tucats/ego/app-cli/ui"
)

type AssetObject struct {
	// The Data being stored in the cache.
	Data []byte

	// The number of times this asset has been accessed from the cache.
	Count int

	// The size of this individual object in bytes
	Size int

	// The last time the asset was accessed. This is used to evict the least
	// recently accessed asset from the cache.
	LastUsed time.Time
}

var (
	// AssetCache is the map that identifies the objects that are in the
	// cache. Each asset has a unique name (typically the endpoint path used
	// to reference it in HTML code).
	AssetCache map[string]AssetObject

	// AssetMux is the mutex used to protect the asset cache from concurrent
	// access.
	AssetMux sync.Mutex

	// masAssetCacheSize is the maximum size of the asset cache in bytes,
	// which is defaults to 5MB.
	maxAssetCacheSize int = 10 * 1024 * 1024

	// assetCacheSize is the current size of the asset cache in bytes.
	assetCacheSize int = 0
)

// Flush the cache of assets being held in memory on behalf of the html services.
func FlushAssetCache() {
	AssetMux.Lock()
	defer AssetMux.Unlock()

	AssetCache = map[string]AssetObject{}
	assetCacheSize = 0

	ui.Log(ui.AssetLogger, "asset.init", ui.A{
		"size": maxAssetCacheSize})
}

// Get the current asset cache size. This is the total number of bytes of data currently
// stored in the cache.
func GetAssetCacheSize() int {
	AssetMux.Lock()
	defer AssetMux.Unlock()

	return assetCacheSize
}

// Get the number of items in the asset cache.
func GetAssetCacheCount() int {
	AssetMux.Lock()
	defer AssetMux.Unlock()

	return len(AssetCache)
}

// normalizeCachePath ensures cache keys always start with "/" so that
// lookupCachedAsset and cacheAsset use the same key for a given asset,
// regardless of whether the caller included a leading slash.
func normalizeCachePath(path string) string {
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}

	return path
}

// initCacheIfNeeded initializes AssetCache when it is nil. Must be called
// with AssetMux already held.
func initCacheIfNeeded(sessionID int) {
	if AssetCache == nil {
		AssetCache = map[string]AssetObject{}

		ui.Log(ui.AssetLogger, "asset.init.session", ui.A{
			"session": sessionID,
			"size":    maxAssetCacheSize})
	}
}

// For a given asset path, look it up in the cache. If found, the asset is returned
// as a byte array. If not found, a nil value is returned. The session id is only
// used for logging purposes.
func lookupCachedAsset(sessionID int, path string) []byte {
	AssetMux.Lock()
	defer AssetMux.Unlock()

	initCacheIfNeeded(sessionID)

	path = normalizeCachePath(path)

	if a, ok := AssetCache[path]; ok {
		a.LastUsed = time.Now()
		a.Count = a.Count + 1
		a.Size = len(a.Data)
		AssetCache[path] = a

		ui.Log(ui.AssetLogger, "asset.loaded", ui.A{
			"session": sessionID,
			"path":    path,
			"size":    len(a.Data)})

		return a.Data
	}

	ui.Log(ui.AssetLogger, "asset.not.found", ui.A{
		"session": sessionID,
		"path":    path})

	return nil
}

// For a given asset path and an asset byte array, store it in the cache. If the cache
// grows too large, then drop objects from the cache, oldest-first.
//
// There is a maximum size of data that is permitted to be cached; items that are too
// large are not stored in cache and must be reloaded from the file system each time
// they are accessed. The maximum size is one half of the total maximum size of the
// cached data, specified by the `maxAssetCacheSize` configuration setting.
func cacheAsset(sessionID int, path string, data []byte) {
	if len(data) > maxAssetCacheSize/2 {
		ui.Log(ui.AssetLogger, "asset.too.large", ui.A{
			"session": sessionID,
			"path":    path,
			"size":    len(data),
			"max":     maxAssetCacheSize})

		return
	}

	AssetMux.Lock()
	defer AssetMux.Unlock()

	initCacheIfNeeded(sessionID)

	path = normalizeCachePath(path)

	a := AssetObject{
		Data:     data,
		Size:     len(data),
		LastUsed: time.Now(),
	}

	// Does it already exist? If so, delete the old object and also subtract
	// the old data size for this path.
	if oldAsset, found := AssetCache[path]; found {
		assetCacheSize = assetCacheSize - len(oldAsset.Data)

		delete(AssetCache, path)
	}

	// Add in the new asset, and increment the asset cache size
	AssetCache[path] = a
	newSize := len(a.Data)
	assetCacheSize = assetCacheSize + newSize

	// If the cache is now too big, delete stuff until it shrinks enough
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

		oldSize := len(AssetCache[oldestAsset].Data)
		assetCacheSize = assetCacheSize - oldSize

		delete(AssetCache, oldestAsset)

		ui.Log(ui.AssetLogger, "asset.purged", ui.A{
			"session": sessionID,
			"path":    path,
			"size":    oldSize,
			"newsize": assetCacheSize})
	}

	ui.Log(ui.AssetLogger, "asset.saved", ui.A{
		"session": sessionID,
		"path":    path,
		"size":    newSize,
		"newsize": assetCacheSize})
}
