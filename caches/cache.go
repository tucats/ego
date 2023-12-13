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

// active is a flag indicating if caching is active or not.
var active = true

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

	go expire(id)

	return cacheList[id]
}

// expire is the go routine launched when a new cache is initialized. It
// sleeps for the "scan" interval, and then locks the cache. It then checks each
// item in the cache to determine if it has expired. If it has expired, it is
// deleted from the cache. Once the scan is complete, the cache is unlocked and
// the flusher goes back to sleep for another scan interval.
func expire(id int) {
	delay, _ := time.ParseDuration(scanTime)

	for {
		time.Sleep(delay)
		cacheLock.Lock()

		if cache, found := cacheList[id]; found {
			count := 0

			ui.Log(ui.CacheLogger, ">>> Cache %s starting expiration scan", cache.ID)

			for key, item := range cache.Items {
				if time.Now().After(item.Expires) {
					count++

					delete(cache.Items, key)
					ui.Log(ui.CacheLogger, ">>> Cache %s deleted expired item: %v", cache.ID, key)
				}
			}

			ui.Log(ui.CacheLogger, ">>> Cache %s expired %d items", cache.ID, count)
		}

		cacheLock.Unlock()
	}
}

// Purge is used to discard all elements of a given cache, identified by an integer key. If
// there is no such cache, no action is taken.
func Purge(id int) {
	cacheLock.Lock()
	defer cacheLock.Unlock()

	if !active {
		return
	}

	if cache, found := cacheList[id]; found {
		ui.Log(ui.CacheLogger, ">>> Cache %s purging %d items", cache.ID, len(cache.Items))

		delete(cacheList, id)
	}
}

// Active enables or disables caching. If caching was active and is now turned off, the in-memory
// cache is deleted.
func Active(flag bool) {
	cacheLock.Lock()
	defer cacheLock.Unlock()

	if flag {
		if !active {
			cacheList = map[int]Cache{}
		}
	} else {
		cacheList = nil
	}

	active = flag
}
