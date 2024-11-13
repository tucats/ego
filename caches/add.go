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

	keyString := fmt.Sprintf("%v", key)
	if len(keyString) > 31 {
		keyString = keyString[:31] + "..."
	}

	ui.Log(ui.CacheLogger, ">>> Cache %s (%s) added item: %v", class(id), cache.ID, keyString)
}
