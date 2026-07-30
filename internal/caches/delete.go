package caches

import (
	"fmt"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/util/strings"
)

// Delete removes a value stored in a cache. The cache is identified by an integer
// value, and the key is any value type that can be used as a map index.
//
// If the value is found in the cache, the function returns true indicating it was
// deleted. If there is no matching item in the cache, the function returns false.
//
// Parameters:
//
//	id		The cache to which the value is added
//	key		The key for the value.
//	value	The value to be added to the cache.
//
// Returns:
//
//	true if the item was found and deleted.
//
// GORTNS-2: if an eviction callback is registered via SetOnEvict, it is invoked
// for the removed item after the cache lock has been released, so the owner can
// release any resource it was holding. Note the structure below: the lock is
// released explicitly rather than with "defer", because a deferred unlock would
// run after the return statement and therefore after the callback, which would
// break the "callback runs unlocked" guarantee SetOnEvict documents.
func Delete(id int, key any) bool {
	if !active {
		return false
	}

	var (
		deleted bool
		evicted map[any]any
	)

	// Read once, outside the lock, whether anyone is listening for evictions.
	watchingEvictions := evictHandler() != nil

	cacheLock.Lock()

	if cache, found := cacheList[id]; found {
		if item, found := cache.Items[key]; found {
			// Capture the value before removing it, but only when somebody is
			// listening for evictions.
			if watchingEvictions {
				evicted = map[any]any{key: item.Data}
			}

			delete(cache.Items, key)

			shortToken := fmt.Sprintf("%v", key)
			if id != SchemaCache && len(shortToken) > 9 {
				shortToken = egostrings.TruncateMiddle(shortToken, cache.MaxWidth)
			}

			ui.Log(ui.CacheLogger, "cache.delete", ui.A{
				"name": class(id),
				"id":   cache.ID,
				"key":  shortToken})

			deleted = true
		}
	}

	cacheLock.Unlock()

	notifyEvictions(id, evicted)

	return deleted
}
