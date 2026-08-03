package caches

import (
	"fmt"
	"time"

	"github.com/tucats/ego/internal/cli/ui"
	egostrings "github.com/tucats/ego/internal/util/strings"
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
func Add(id int, key any, value any) {
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

	// Not replaceable, so check to see if the cache is too darn full already.
	// If so, we reject this item and hope that cache sweeping helps later
	// with aging out old items. We don't do a sweep now because under a uniform
	// cache access load, the cache benefit is completely lost in the cost of
	// constant ejection churn.
	if cacheSize := len(cache.Items); cacheSize >= cache.MaxSize {
		// If we haven't logged this message yet, do so now. cacheList[id] must be
		// updated explicitly here: cacheList is a map[int]Cache (not a map of
		// pointers), so "cache" is a value copy, and HasLogged is a plain bool
		// field rather than a reference type like the Items map -- without this
		// write-back, the flag would reset to false the moment this function
		// returns, and every over-capacity Add() would log again.
		if !cache.HasLogged {
			cache.HasLogged = true
			cacheList[id] = cache

			if ui.IsActive(ui.CacheLogger) {
				ui.Log(ui.CacheLogger, "cache.full", ui.A{
					"name":  class(id),
					"id":    cache.ID,
					"count": len(cache.Items)})
			}
		}
		// Can't add to the cache, so no further work here.
		return
	} else {
		// If we have logged a full cache, and the cache size is now below 95% of the
		// maximum size, then reset the logged flag so that we can log again if it
		// fills up again. Same write-back requirement as above.
		if cache.HasLogged && (float64(cacheSize) < float64(cache.MaxSize)*0.95) {
			cache.HasLogged = false
			cacheList[id] = cache
		}
	}

	delay := cache.Expiration
	item := Item{
		Data:    value,
		Expires: time.Now().Add(delay),
	}

	cache.Items[key] = item

	if ui.IsActive(ui.CacheLogger) {
		shortToken := fmt.Sprintf("%v", key)
		if id != SchemaCache && len(shortToken) > 9 {
			shortToken = egostrings.TruncateMiddle(shortToken, cache.MaxWidth)
		}

		ui.Log(ui.CacheLogger, "cache.added", ui.A{
			"name":    class(id),
			"id":      cache.ID,
			"expires": item.Expires.Format(timeFormat),
			"key":     shortToken})
	}
}
