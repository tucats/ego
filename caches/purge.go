package caches

import (
	"sort"

	"github.com/tucats/ego/app-cli/ui"
)

// Purge is used to discard all elements of a given cache, identified by an integer key. If
// there is no such cache, no action is taken.
func Purge(id int) {
	cacheLock.Lock()
	defer cacheLock.Unlock()

	if !active {
		return
	}

	if cache, found := cacheList[id]; found {
		ui.Log(ui.CacheLogger, "cache.purge", ui.A{
			"name":  class(id),
			"id":    cache.ID,
			"count": len(cache.Items)})

		delete(cacheList, id)
	}
}

// PurgeAll purges all defined caches. It uses the map of cache ID to name to get the
// list of cache ID values.
func PurgeAll() {
	ui.Log(ui.CacheLogger, "cache.purge.all", nil)

	// Get the list of cache ID values and sort them in ascending order. This ensures
	// that the order they are printed in the log is consistent across different runs
	// of the application.
	keys := make([]int, 0, len(cacheList))
	for id := range cacheList {
		keys = append(keys, id)
	}

	sort.Ints(keys)

	// Now that we know the range of all ID values, purge them in ascending order.
	for _, id := range keys {
		Purge(id)
	}
}
