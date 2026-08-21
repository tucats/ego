package services

import (
	"strings"
	"sync"
	"time"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokenizer"
	"github.com/tucats/ego/internal/router"
	egostrings "github.com/tucats/ego/internal/util/strings"
)

// Define a cache element. This keeps a copy of the compiler instance
// and the bytecode used to represent each service compilation. The Age
// is exported as a variable that shows when the item was put in the
// cache, and is used to retire items from the cache when it gets full.
type CachedCompilationUnit struct {
	Age   time.Time
	b     *bytecode.ByteCode
	t     *tokenizer.Tokenizer
	s     *symbols.SymbolTable
	Route *router.Route
	Count int
	Size  int
}

// ServiceCache is a map that contains compilation data for previously-
// compiled service handlers written in the Ego language.
var ServiceCache = map[string]*CachedCompilationUnit{}
var serviceCacheMutex sync.Mutex

// MaxCachedEntries is the maximum number of items allowed in the service
// cache before items start to be aged out (oldest first).
var MaxCachedEntries = 20

// setupServiceCache ensures that the service cache is configured.
func setupServiceCache() {
	serviceCacheMutex.Lock()

	if MaxCachedEntries < 0 {
		txt := settings.Get(defs.MaxServiceCacheSizeSetting)

		n, err := egostrings.Atoi(txt)
		if err != nil {
			ui.Log(ui.ServicesLogger, "services.invalid.ignored", ui.A{
				"name":  defs.MaxServiceCacheSizeSetting,
				"value": txt})
		} else {
			MaxCachedEntries = n
		}
	}

	serviceCacheMutex.Unlock()
}

// FlushServiceCache will flush the service cache. This is used when the
// user requests a flush operation via the /admin/flush endpoint. This
// is thread-safe, and resets the cache structure to its initial state.
func FlushServiceCache() {
	serviceCacheMutex.Lock()
	defer serviceCacheMutex.Unlock()

	// First, scan over everything in the cache and reset it's first-run counter
	for _, item := range ServiceCache {
		item.Route.NeedsLock(true)
	}

	// Now we can dump the cache and reset to a fresh empty map.
	ServiceCache = map[string]*CachedCompilationUnit{}
}

// Update the cache entry for a given endpoint with the supplied compiler, bytecode, and tokens. If necessary,
// age out the oldest cached item (based on last time-of-access) from the cache to keep it within the maximum
// cache size.
//
// This function used to write to (and, in the aging loop below, delete from)
// the ServiceCache map without holding serviceCacheMutex at all. Two 
// goroutines calling this at the same time -- for example, the first two
// concurrent requests for a service that isn't cached yet -- were
// both writing to the same Go map with no synchronization, which is
// undefined behavior and can crash or hang the process. See also the fix in
// getCachedService below, which had a matching unprotected read.
func addToCache(session *router.Session, endpoint string, code *bytecode.ByteCode, tokens *tokenizer.Tokenizer) {
	serviceCacheMutex.Lock()
	defer serviceCacheMutex.Unlock()

	ui.Log(ui.ServicesLogger, "services.cache.add", ui.A{
		"session":  session,
		"endpoint": endpoint})

	ServiceCache[endpoint] = &CachedCompilationUnit{
		Route: session.Route,
		Age:   time.Now(),
		b:     code,
		t:     tokens,
		s:     nil, // Gets written here after first successful execution
		Count: 1,   // We count the initial load of the service as a usage.
		Size:  code.Size(),
	}

	// Is the cache too large? If so, throw out the oldest
	// item from the cache.
	for len(ServiceCache) > MaxCachedEntries {
		var (
			key       string
			route     *router.Route
			oldestAge float64
		)

		for k, v := range ServiceCache {
			thisAge := time.Since(v.Age).Seconds()
			if thisAge > oldestAge {
				key = k
				route = v.Route
				oldestAge = thisAge
			}
		}

		// The route that goes with the oldest item needs to have its first-use count reset.
		route.NeedsLock(true)

		// Delete the item from the cache and report it.
		delete(ServiceCache, key)
		ui.Log(ui.ServicesLogger, "services.cache.aged", ui.A{
			"endpoint": key,
			"session":  session})
	}
}

// updateCacheUsage updates the metadata for the service cache entry to reflect
// that the service was reused. In particular, this updates the timestamp used
// to support aging LRU cache entries, and the count of usages of this service.
//
// This used to read and write the ServiceCache map (cachedItem.Age,
// cachedItem.Count) without holding serviceCacheMutex, even though every other
// function in this file that touches ServiceCache does take that lock. A
// busy server handling many requests to the same endpoint at once (exactly
// the kind of load examples/swarm.ego generates against /services/factor)
// would have many goroutines calling this at the same time, all reading and
// writing the same map and the same *CachedCompilationUnit fields with no
// synchronization at all -- a textbook concurrent-map-access data race, and
// on the shared cachedItem.Count field, a lost-update race too (both
// goroutines could read the same old value before either writes back the
// incremented one).
func updateCacheUsage(endpoint string) {
	serviceCacheMutex.Lock()
	defer serviceCacheMutex.Unlock()

	if cachedItem, ok := ServiceCache[endpoint]; ok {
		cachedItem.Age = time.Now()
		cachedItem.Count++
	}
}

func updateCachedServiceSymbols(sessionID int, endpoint string, symbolTable *symbols.SymbolTable) {
	serviceCacheMutex.Lock()
	defer serviceCacheMutex.Unlock()

	if cachedItem, ok := ServiceCache[endpoint]; ok && cachedItem.s == nil {
		cachedItem.s = symbolTable
		count := 0

		for _, k := range symbolTable.Names() {
			if !strings.HasPrefix(k, defs.InvisiblePrefix) {
				count++
			}
		}

		ui.Log(ui.ServicesLogger, "services.pkg.saved", ui.A{
			"session":  sessionID,
			"endpoint": endpoint,
			"count":    count})
	}
}

// getCachedService gets a service by endpoint name. This will either be retrieved from the
// cache, or read from disk, compiled, and then added to the cache.
func getCachedService(session *router.Session, endpoint string, debug bool, file string, symbolTable *symbols.SymbolTable) (serviceCode *bytecode.ByteCode, tokens *tokenizer.Tokenizer, err error) {
	sessionID := session.ID

	// This lookup used to read the ServiceCache map directly with no
	// lock held (`if cachedItem, ok := ServiceCache[endpoint]; ok {`), even
	// though addToCache below writes to the very same map. Take the lock just
	// long enough to do the lookup and copy out the *CachedCompilationUnit
	// pointer, then release it -- the rest of this function only touches
	// fields on that one cached item (protected individually by
	// updateCacheUsage and by Merge's own lock on cachedItem.s), or calls
	// addToCache/compileAndCacheService, which take the map lock themselves
	// when they need it. Holding serviceCacheMutex for the whole function
	// would risk a self-deadlock, since updateCacheUsage (called below) also
	// acquires it.
	serviceCacheMutex.Lock()
	cachedItem, found := ServiceCache[endpoint]
	serviceCacheMutex.Unlock()

	// Is this endpoint already in the cache of compiled services?
	if found {
		serviceCode = cachedItem.b
		tokens = cachedItem.t

		updateCacheUsage(endpoint)
		ui.Log(ui.ServicesLogger, "services.cache.use", ui.A{
			"session":  sessionID,
			"endpoint": endpoint})

		if debug {
			ui.Log(ui.ServicesLogger, "service.debug.enabled", ui.A{
				"session":  sessionID,
				"endpoint": endpoint})
		}

		// cachedItem.s is written once, under serviceCacheMutex, by
		// updateCachedServiceSymbols the first time this endpoint finishes
		// executing (see below). Read it back under the same lock rather than
		// touching cachedItem.s directly here, since another goroutine
		// finishing that first execution concurrently could be writing it at
		// the exact same moment.
		serviceCacheMutex.Lock()
		cachedServiceSymbols := cachedItem.s
		serviceCacheMutex.Unlock()

		if count := symbolTable.Merge(cachedServiceSymbols); count > 0 {
			ui.Log(ui.ServicesLogger, "services.pkg.loaded", ui.A{
				"session": sessionID,
				"name":    cachedServiceSymbols.Name,
				"count":   count})
		}
	} else {
		serviceCode, tokens, err = compileAndCacheService(session, endpoint, file, symbolTable)
		// If it compiled successfully and we are caching, then put it in the cache. If we
		// are in debug mode, then we store the associated token stream; if not, then no tokens
		// are stored.
		if err == nil && MaxCachedEntries > 0 {
			var cachedTokens *tokenizer.Tokenizer

			if debug {
				cachedTokens = tokens
			}

			addToCache(session, endpoint, serviceCode, cachedTokens)
		}
	}

	return serviceCode, tokens, err
}
