package caches

import (
	"fmt"

	"github.com/tucats/ego/app-cli/ui"
)

// Delete removes a value stored in a cache. The cache is identified by an integer
// value, and the key is any value type that can be used as a map index.
//
// If the value is found in the cache, the function returns true indicating it was
// deleted. If there is no matching item in the cache, the function returns false.
func Delete(id int, key interface{}) bool {
	if !active {
		return false
	}

	cacheLock.Lock()
	defer cacheLock.Unlock()

	if cache, found := cacheList[id]; found {
		if _, found := cache.Items[key]; found {
			delete(cache.Items, key)

			keyString := fmt.Sprintf("%v", key)
			if len(keyString) > 31 {
				keyString = keyString[:31] + "..."
			}

			ui.Log(ui.CacheLogger, ">>> Cache %s (%s) deleted item: %v", class(id), cache.ID, keyString)

			return true
		}
	}

	return false
}
