package caches

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
)

type Item struct {
	Data    interface{}
	Expires time.Time
}

type Cache struct {
	ID    uuid.UUID
	Items map[interface{}]Item
}

const (
	DSNCache int = iota
	AuthCache
	UserCache
)

// cacheList is the list of all the caches, indexed by an integer value. It
// is initially empty, and only gets values when an Add operation is done on
// a given cache ID.
var cacheList = map[int]Cache{}

// cacheLock is a mutex used to protect the cache. Attempts to read the cache
// (the most common operation) can be done with a read lock, which allowed
// concurrent read access to the map. Any changes to the map must be done
// using a read/write lock which serializes access until the operation is
// complete.
var cacheLock sync.RWMutex

// scanTime is the delay between scans of a given cache to see if any items
// have expired. If the value is 60s, then it means the cache scans for expired
// items once a minute.
var scanTime = "60s"

// expireTime is the amount of time an item is allowed to remain in the cache
// after it is added (or updated). By default, this is the same as the scanTime,
// so items normally are tossed out after a maximum of two minutes (scan time
// plus expire time).
var expireTime = "60s"

// newCache is used to create a new cache, identified by an integer value. The
// cache is initialized and a flushing thread is started to scan the cache for
// expired entries. This operation is not done directly by the user, but is
// called by the Add() function the first time that a cache ID number is used.
func newCache(id int) Cache {
	cacheID, _ := uuid.NewUUID()

	cacheList[id] = Cache{
		ID:    cacheID,
		Items: map[interface{}]Item{},
	}

	ui.Log(ui.CacheLogger, ">>> Cache %s created", cacheID)

	go cacheFlusher(id)

	return cacheList[id]
}

// cacheFlusher is the go routine launched when a new cache is initialized. It
// sleeps for the "scan" interval, and then locks the cache. It then checks each
// item in the cache to determine if it has expired. If it has expired, it is
// deleted form the cache. Once the scan is complete, the cache is unlocked and
// the flusher goes back to sleep for another scan interval.
func cacheFlusher(id int) {
	delay, _ := time.ParseDuration(scanTime)

	for {
		time.Sleep(delay)
		cacheLock.Lock()

		if cache, found := cacheList[id]; found {
			ui.Log(ui.CacheLogger, ">>> Cache %s starting expiration scan", cache.ID)

			for key, item := range cache.Items {
				if time.Now().After(item.Expires) {
					delete(cache.Items, key)
					ui.Log(ui.CacheLogger, ">>> Cache %s deleted expired item: %v", cache.ID, key)
				}
			}
		}

		cacheLock.Unlock()
	}
}

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
	cacheLock.RLock()
	defer cacheLock.RUnlock()

	if cache, found := cacheList[id]; found {
		if item, found := cache.Items[key]; found {
			return item.Data, true
		}
	}

	return nil, false
}

// Add adds a value to a cache. The cache is identified using by an integer value,
// and the item is represented by a key and value. The item will remain in the cache
// until it expires.
func Add(id int, key interface{}, value interface{}) {
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
}
