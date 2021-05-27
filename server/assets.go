package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/app-cli/ui"
)

type asset struct {
	data     []byte
	lastUsed time.Time
}

var assetCache map[string]asset
var assetMux sync.Mutex
var maxAssetCache int = 100

func findAsset(path string) []byte {
	assetMux.Lock()
	defer assetMux.Unlock()

	if assetCache == nil {
		assetCache = map[string]asset{}

		ui.Debug(ui.ServerLogger, "Initialized asset cache for %d items", maxAssetCache)
	}

	if a, ok := assetCache[path]; ok {
		a.lastUsed = time.Now()
		assetCache[path] = a

		ui.Debug(ui.InfoLogger, "Asset loaded from cache: %s", path)

		return a.data
	}

	ui.Debug(ui.InfoLogger, "Asset not found in cache: %s", path)

	return nil
}

func saveAsset(path string, data []byte) {
	assetMux.Lock()
	defer assetMux.Unlock()

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	a := asset{
		data:     data,
		lastUsed: time.Now(),
	}
	assetCache[path] = a

	if len(assetCache) > maxAssetCache {
		oldestAsset := ""
		oldestTime := time.Now()

		for k, v := range assetCache {
			age := v.lastUsed.Sub(oldestTime)
			if age < 0 {
				oldestTime = v.lastUsed
				oldestAsset = k
			}
		}

		delete(assetCache, oldestAsset)
		ui.Debug(ui.InfoLogger, "Asset purged from cache: %s", oldestAsset)
	}

	ui.Debug(ui.InfoLogger, "Asset saved to cache: %s", path)
}

func AssetsHandler(w http.ResponseWriter, r *http.Request) {
	var err error

	path := r.URL.Path

	data := findAsset(path)
	if data == nil {
		for strings.HasPrefix(path, ".") || strings.HasPrefix(path, "/") {
			path = path[1:]
		}

		root := persistence.Get("ego.path")
		fn := filepath.Join(root, "lib/services", path)

		ui.Debug(ui.InfoLogger, "Asset loaded from file: %s", fn)

		data, err = ioutil.ReadFile(fn)
		if err != nil {
			msg := fmt.Sprintf(`{"err": "%s"}`, err.Error())

			ui.Debug(ui.InfoLogger, "Server asset load error: %s", err.Error())
			w.WriteHeader(400)
			_, _ = w.Write([]byte(msg))

			return
		}

		saveAsset(path, data)
	}

	w.WriteHeader(200)
	_, _ = w.Write(data)
}
