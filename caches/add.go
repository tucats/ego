package caches

import (
	"time"

	"github.com/tucats/ego/app-cli/ui"
)

// Add adds a value to a cache. The cache is identified using by an integer value,
// and the item is represented by a key and value. The item will remain in the cache
// until it expires.
func Add(id int, key interface{}, value interface{}) {
	if !active {
		return
	}

	cacheLock.Lock()
	defer cacheLock.Unlock()

	cache, found := cacheList[id]
	if !found {
		cache = newCache(id)
	} else {
		delete(cache.Items, key)
	}

	delay, _ := time.ParseDuration(expireTime)
	item := Item{
		Data:    value,
		Expires: time.Now().Add(delay),
	}

	cache.Items[key] = item

	ui.Log(ui.CacheLogger, ">>> Cache %s added item: %v", cache.ID, key)
}
