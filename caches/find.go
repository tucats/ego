package caches

import (
	"fmt"

	"github.com/tucats/ego/app-cli/ui"
)

// Find returns a value stored in a cache. The cache is identified by an integer
// value, and the key is any value type that can be used as a map index.
//
// If the value is found in the cache, it is returned as the result, along with the
// flag "true" to indicate it was found. If it was not found, a nil is returned with
// the flag "false" to indicate it was not in the cache.
//
// Note that the value might have never been inserted into the cache, or the cache
// item may have expired. By default, the cache is scanned every 60 seconds and any
// expired items are removed.
func Find(id int, key interface{}) (interface{}, bool) {
	if !active {
		return nil, false
	}

	cacheLock.RLock()
	defer cacheLock.RUnlock()

	if cache, found := cacheList[id]; found {
		if item, found := cache.Items[key]; found {

			keyString := fmt.Sprintf("%v", key)
			if len(keyString) > 31 {
				keyString = keyString[:31] + "..."
			}

			ui.Log(ui.CacheLogger, ">>> Cache %s (%s) located item: %v", class(id), cache.ID, keyString)

			return item.Data, true
		}
	}

	return nil, false
}
