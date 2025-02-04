package caches

import (
	"fmt"
	"time"

	"github.com/tucats/ego/app-cli/ui"
)

// Add adds a value to a cache. The cache is identified using by an integer value,
// and the item is represented by a key and value. The item will remain in the cache
// until it expires.
//
// Parameters:
//
//	id		The cache to which the value is added
//	key		The key for the value.
//	value	The value to be added to the cache.
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

	shortToken := fmt.Sprintf("%v", key)
	if len(shortToken) > 9 {
		shortToken = shortToken[:4] + "..." + shortToken[len(shortToken)-4:]
	}

	ui.Log(ui.CacheLogger, "cache.added", ui.A{
		"name": class(id),
		"id":   cache.ID,
		"key":  shortToken})
}
