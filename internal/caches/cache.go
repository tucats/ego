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

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	egostrings "github.com/tucats/ego/internal/util/strings"
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
// The MaxSize is the maximum number of items that can be stored in the cache. If the cache
// is full, the oldest item is removed to make room for the new item. The default is 1,000
// items but this default can be overridden by the ego.server.cache.maxsize setting.
type Cache struct {
	ID         int32
	MaxWidth   int
	MaxSize    int
	HasLogged  bool
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

	// Pending WebAuthn ceremony session data, keyed by a UUID nonce that is
	// round-tripped to the browser as a short-lived cookie.  Items expire after
	// five minutes which is ample time for a user to complete the ceremony.
	WebAuthnChallengeCache

	// OAuthCodeCache holds short-lived OAuth2 authorization codes for the
	// Authorization Code flow. Each entry maps the opaque code string to a
	// PendingAuthorization struct. Items are single-use: consumeCode() deletes
	// the cache entry immediately after it is read. The default TTL is five
	// minutes, matching the recommended maximum from the OAuth2 specification.
	OAuthCodeCache

	// OAuthRefreshCache holds OAuth2 refresh tokens, each mapping an opaque
	// random string to a RefreshTokenData struct. The default TTL is 24 hours
	// but is overridden by the ego.server.oauth.as.refresh.expiration setting.
	OAuthRefreshCache

	// OAuthJWTCache holds the results of validated JWT Bearer tokens presented
	// by external OAuth2 clients. Keyed on the raw JWT string; each entry is a
	// *JWTCacheEntry (defined in server/oauth) containing the extracted username
	// and permission list. Items expire according to ego.server.oauth.jwks.cache.ttl
	// (default 1 hour), which should be set no longer than the IdP's token lifetime.
	// Using this cache avoids repeating JWKS signature verification on every request.
	OAuthJWTCache
)

// By default, this is the maximum number of items that can be stored in a cache. If the
// cache is full, no new items are added to the cache. The default is 1,000 items but
// this default can be overridden by the ego.server.cache.maxsize setting with a
// positive integer value. A value of zero means use the default value here.
var MaxCacheSize = 1000

// OnPurge is an optional callback that is invoked by Purge after a cache has
// been discarded. It receives the same cache class integer that was passed to
// Purge. The server/cluster package sets this at startup so that every local
// cache eviction automatically triggers a broadcast to cluster peers.
//
// Setting OnPurge to nil (the default) is a no-op and incurs no overhead.
var OnPurge func(int)

// onEvict holds the optional eviction callback registered via SetOnEvict, and
// onEvictMutex protects it.
//
// The mutex is needed because the callback is read by the background expiration
// goroutines, which run concurrently with whatever goroutine registers it. In
// production the handler is installed from an init function long before any
// request arrives, so an unsynchronized read would happen to work -- but "happens
// to work" is not good enough for a variable two goroutines can touch. Go's race
// detector flags an unsynchronized global like that even when the timing works
// out, and it is right to: the compiler is free to assume no other goroutine is
// writing, so there is no guarantee a later write is ever observed.
//
// A read-write mutex is used rather than a plain one because reads vastly
// outnumber the single write, and RLock lets concurrent evictions proceed without
// blocking each other.
var (
	onEvictMutex sync.RWMutex
	onEvict      func(id int, key any, value any)
)

// SetOnEvict registers a callback invoked once for each individual item removed
// from a cache, whether it was removed because it expired or because Delete was
// called on it. The callback receives the cache class, the item's key, and the
// value that was stored. Passing nil removes any existing handler.
//
// GORTNS-2: this exists because some cached values own a resource that has to be
// released, not just memory the garbage collector will reclaim. A debug session,
// for example, is a cached value whose goroutine is parked waiting for the next
// command; dropping the map entry leaves that goroutine blocked forever. Removing
// the entry frees the cache slot but not the resource, so there has to be a way
// for the owner of the value to be told.
//
// Two guarantees an implementation can rely on:
//
//   - The callback is invoked AFTER the cache's internal lock has been released.
//     That means it may safely call back into this package (or do anything else
//     that takes a lock) without deadlocking against the cache.
//   - It is invoked synchronously, on whichever goroutine performed the eviction.
//     Implementations must therefore return promptly and must not block; if real
//     work is needed, hand it off.
func SetOnEvict(handler func(id int, key any, value any)) {
	onEvictMutex.Lock()
	defer onEvictMutex.Unlock()

	onEvict = handler
}

// evictHandler returns the currently registered eviction callback, or nil if
// there is none. Callers use it instead of reading the variable directly so that
// every read is protected by the mutex.
func evictHandler() func(id int, key any, value any) {
	onEvictMutex.RLock()
	defer onEvictMutex.RUnlock()

	return onEvict
}

// notifyEvictions invokes the eviction callback for a batch of removed items. It
// exists so the call sites can gather evicted entries while holding the cache
// lock and then deliver the notifications after releasing it, which is what makes
// the "callback runs unlocked" guarantee above true.
func notifyEvictions(id int, evicted map[any]any) {
	if len(evicted) == 0 {
		return
	}

	handler := evictHandler()
	if handler == nil {
		return
	}

	for key, value := range evicted {
		handler(id, key, value)
	}
}

// Map the cache classes to a string representation for easier logging.
var cacheClass = map[int]string{
	DSNCache:               "Data Source Name",
	AuthCache:              "Authorization",
	UserCache:              "Authentication",
	TokenCache:             "Decrypted Token",
	BlacklistCache:         "Token Blacklist",
	SchemaCache:            "Table Schema",
	SymbolTableCache:       "Symbol Table",
	DebugSessionCache:      "Debug Session",
	WebAuthnChallengeCache: "Web Challenge",
	OAuthCodeCache:         "OAuth2 Auth Code",
	OAuthRefreshCache:      "OAuth2 Refresh Token",
	OAuthJWTCache:          "OAuth2 JWT",
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

	// Get the default expiration time for caches from the settings database.
	// If there is no value (or it is not a valid positive integer) then use
	// the default of 1,000 items. This is a global setting that applies to all
	// caches.
	if size := settings.GetInt(defs.ServerMaxCacheSizeSetting); size > 0 {
		MaxCacheSize = size
	}

	cacheList[id] = Cache{
		ID:         cacheID,
		Items:      map[any]Item{},
		Expiration: expiration,
		MaxWidth:   maxWidth,
		MaxSize:    MaxCacheSize,
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

		if !sweepExpired(id) {
			return
		}
	}
}

// sweepExpired removes every expired item from one cache and reports each removal
// to the eviction callback. It returns true if the cache still exists (so the caller should
// keep scanning) and false if the cache is gone, in which case it has already
// cleared the "expiration thread running" flag so a future cache can start a
// fresh scan.
//
// This is a separate function from expire, rather than the body of its loop, for
// two reasons: it keeps the lock handling in one readable place, and it lets the
// tests drive a single sweep deterministically instead of waiting out a real
// scan interval.
func sweepExpired(id int) bool {
	// GORTNS-2: expired items are collected here and reported to the eviction callback only
	// after the cache lock is released, so an eviction callback can safely take
	// locks of its own (including this package's) without deadlocking against us.
	// That is why "evicted" is declared before the lock is taken -- it has to
	// outlive the locked section.
	var evicted map[any]any

	// Read once, outside the lock, whether anyone is listening. Capturing the
	// values of evicted items costs nothing to skip when nobody is.
	watchingEvictions := evictHandler() != nil

	cacheLock.Lock()

	cache, found := cacheList[id]
	if !found {
		// Cache doesn't exist any more, so stop the expiration scan goroutine.
		ui.Log(ui.CacheLogger, "cache.scan.not.found", ui.A{
			"name": class(id),
			"id":   id})

		// Clear the flag indicating an expiration thread is running, so it can
		// be restarted if the cache goes active again.
		expirationThreadRunning[id] = false

		cacheLock.Unlock()

		return false
	}

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

			// Remember the value before dropping it, but only when somebody is
			// actually listening for evictions.
			if watchingEvictions {
				if evicted == nil {
					evicted = map[any]any{}
				}

				evicted[key] = item.Data
			}

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

	cacheLock.Unlock()

	// Now that the lock is released, tell the owner about each evicted item so it
	// can release whatever the value was holding on to.
	notifyEvictions(id, evicted)

	return true
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
