// The cache package provides a simple in-memory cache implementation with support for expiration.
// This is used internally for small-to-medium objects that are costly to retrieve, typically from
// a database. The default is that items stay in the cache for up to two minutes after last use,
// but the expiration can be set explicitly on an individual cache class before items are added.
//
// CAche access is thread-safe, and requires only to identify the cache by it's cache class, which
// is an integer value. The cache classes used by Ego are pre-defined, but users can create
// their own cache classes.
//
// The CACHES logging class will record when an item is added, searched, removed, or purged from
// the cache. The /admin/caches endpoint will report on the pre-defined cache class types.
package caches

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
)

// Item represents a value stored in the cache along with its expiration time.
type Item struct {
	Data    any
	Expires time.Time
}

// Cache represents a cache that can store and retrieve values with an expiration time.
// The MaxWidth specifies the maximum size of the key value string that will be reported
// in the log (to prevent excessive log output). The default is 40 characters, except
// tokens which are limited to the first and last four characters.
type Cache struct {
	ID         int32
	MaxWidth   int
	Expiration time.Duration
	Items      map[any]Item
}

// Class ID values for pre-defined cache classes.
const (
	// All information know about a data source name and its permissions.
	DSNCache int = iota

	// Information about the permissions for a given user.
	AuthCache

	// A general-purpose cache available to user code.
	UserCache

	// A cache of decrypted tokens.
	TokenCache

	// List of token UUIDS and their blacklist status.
	BlacklistCache

	// For a given user/dsn/table, the table schema information.
	SchemaCache

	// For a given /admin/run session, the stored symbol table.
	SymbolTableCache

	// For a given /admin/run session, the stored debug session information.
	DebugSessionCache
)

// Map the cache classes to a string representation for easier logging.
var cacheClass = map[int]string{
	DSNCache:          "Data Source Name",
	AuthCache:         "Authorization",
	UserCache:         "Authentication",
	TokenCache:        "Decrypted Token",
	BlacklistCache:    "Token Blacklist",
	SchemaCache:       "Table Schema",
	SymbolTableCache:  "Symbol Table",
	DebugSessionCache: "Debug Session",
}

// Default time format for logging expiration times.
var timeFormat = time.StampMilli

// Sequence number used for unique cache ID values.
var sequenceNumber atomic.Int32

// active is a flag indicating if caching is active or not.
var active = true

// cacheList is the list of all the caches, indexed by an integer value. It
// is initially empty, and only gets values when an Add operation is done on
// a given cache ID.
var cacheList = map[int]Cache{}

// expirationThreadRunning is a map that indicates if an expiration scan has been started
// for a given cache ID. This scan is started the first time the cache is created,
// and is turned off if the cache is removed from the cache list.
var expirationThreadRunning = map[int]bool{}

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
	cacheID := sequenceNumber.Add(1)

	// Set the max width of keys when logging. Default is 40 characters, but
	// for the TokenCache the key is the token value so we only show the first
	// and last few characters with a max width of 10 characters.
	maxWidth := 40
	if id == TokenCache {
		maxWidth = 10
	}

	expiration, _ := time.ParseDuration(expireTime)

	cacheList[id] = Cache{
		ID:         cacheID,
		Items:      map[any]Item{},
		Expiration: expiration,
		MaxWidth:   maxWidth,
	}

	ui.Log(ui.CacheLogger, "cache.created", ui.A{
		"name": class(id),
		"id":   cacheList[id].ID})

	// Start a goroutine to scan the cache for expired entries.
	if !expirationThreadRunning[id] {
		expirationThreadRunning[id] = true

		go expire(id, cacheID)
	}

	return cacheList[id]
}

// Produce the cache class name for a given cache ID.
func class(id int) string {
	name, found := cacheClass[id]
	if !found {
		name = fmt.Sprintf("unknown(%d)", id)
	}

	return name
}

// expire is the go routine launched when a new cache is initialized. It
// sleeps for the "scan" interval, and then locks the cache. It then checks each
// item in the cache to determine if it has expired. If it has expired, it is
// deleted from the cache. Once the scan is complete, the cache is unlocked and
// the flusher goes back to sleep for another scan interval.
//
// When the scan detects that the cache no longer exists (presumably because it
// was explicitly deleted), it stops the expiration scan goroutine.
func expire(id int, cacheID int32) {
	delay, _ := time.ParseDuration(scanTime)

	delayText := delay.String()
	if strings.HasSuffix(delayText, "m0s") {
		delayText = strings.TrimSuffix(delayText, "0s")
	}

	ui.Log(ui.CacheLogger, "cache.scan.launch", ui.A{
		"name":  class(id),
		"id":    cacheID,
		"delay": delayText})

	for {
		time.Sleep(delay)
		cacheLock.Lock()

		if cache, found := cacheList[id]; found {
			count := 0

			for key, item := range cache.Items {
				if time.Now().After(item.Expires) {
					if count == 0 {
						ui.Log(ui.CacheLogger, "cache.scan.start", ui.A{
							"name": class(id),
							"time": time.Now().Format(timeFormat),
							"id":   cache.ID})
					}

					count++

					delete(cache.Items, key)

					shortToken := fmt.Sprintf("%v", key)
					if id != SchemaCache && len(shortToken) > 9 {
						shortToken = egostrings.TruncateMiddle(shortToken, cache.MaxWidth)
					}

					ui.Log(ui.CacheLogger, "cache.scan.delete", ui.A{
						"name":    class(id),
						"id":      cache.ID,
						"expired": item.Expires.Format(timeFormat),
						"key":     shortToken})
				}
			}

			if count > 0 {
				ui.Log(ui.CacheLogger, "cache.scan.delete.count", ui.A{
					"name":  class(id),
					"id":    cacheList[id].ID,
					"count": count})
			}
		} else {
			// Cache doesn't exist any more, so stop the expiration scan goroutine.
			ui.Log(ui.CacheLogger, "cache.scan.not.found", ui.A{
				"name": class(id),
				"id":   id})

			// Clear the flag indicating an expiration thread is running, so it can
			// be restarted if the cache goes active again.
			expirationThreadRunning[id] = false

			cacheLock.Unlock()

			return
		}

		cacheLock.Unlock()
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

// For a given cache class, returns the number of items currently in the cache.
func Size(id int) int {
	cacheLock.RLock()
	defer cacheLock.RUnlock()

	if cache, found := cacheList[id]; found {
		return len(cache.Items)
	}

	return 0
}

// SetExpiration overrides the default expiration for a given cache class.
func SetExpiration(id int, duration string) error {
	expiration, err := time.ParseDuration(duration)
	if err != nil {
		return errors.ErrInvalidDuration.Context(duration)
	}

	cacheLock.Lock()
	defer cacheLock.Unlock()

	cache, found := cacheList[id]
	if !found {
		cache = newCache(id)
	}

	// Update the expiration value and put it back in the cache list.
	cache.Expiration = expiration
	cacheList[id] = cache

	return nil
}
