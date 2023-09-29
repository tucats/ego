package services

import (
	"strconv"
	"sync"
	"time"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
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
	Count int
}

// ServiceCache is a map that contains compilation data for previously-
// compiled service handlers written in the Ego language.
var ServiceCache = map[string]*CachedCompilationUnit{}
var serviceCacheMutex sync.Mutex

// MaxCachedEntries is the maximum number of items allowed in the service
// cache before items start to be aged out (oldest first).
var MaxCachedEntries = 10

// setupServiceCache ensures that the service cache is configured.
func setupServiceCache() {
	serviceCacheMutex.Lock()

	if MaxCachedEntries < 0 {
		txt := settings.Get(defs.MaxCacheSizeSetting)
		MaxCachedEntries, _ = strconv.Atoi(txt)
	}

	serviceCacheMutex.Unlock()
}

// FlushServiceCache will flush the service cache. This is used when the
// user requests a flush operation via the /admin/flush endpoint. This
// is thread-safe, and resets the cache structure to its initial state.
func FlushServiceCache() {
	serviceCacheMutex.Lock()
	defer serviceCacheMutex.Unlock()

	ServiceCache = map[string]*CachedCompilationUnit{}
}

// Update the cache entry for a given endpoint with the supplied compiler, bytecode, and tokens. If necessary,
// age out the oldest cached item (based on last time-of-access) from the cache to keep it within the maximum
// cache size.
func addToCache(session int, endpoint string, code *bytecode.ByteCode, tokens *tokenizer.Tokenizer) {
	ui.Log(ui.ServicesLogger, "[%d] Caching compilation unit for %s", session, endpoint)

	ServiceCache[endpoint] = &CachedCompilationUnit{
		Age:   time.Now(),
		b:     code,
		t:     tokens,
		s:     nil, // Will be filled in at the end of successful execution.
		Count: 1,   // We count the initial load of the service as a usage.
	}

	// Is the cache too large? If so, throw out the oldest
	// item from the cache.
	for len(ServiceCache) > MaxCachedEntries {
		key := ""
		oldestAge := 0.0

		for k, v := range ServiceCache {
			thisAge := time.Since(v.Age).Seconds()
			if thisAge > oldestAge {
				key = k
				oldestAge = thisAge
			}
		}

		delete(ServiceCache, key)
		ui.Log(ui.ServicesLogger, "[%d] Endpoint %s aged out of cache", session, key)
	}
}

func deleteService(endpoint string) {
	serviceCacheMutex.Lock()
	defer serviceCacheMutex.Unlock()

	delete(ServiceCache, endpoint)
}

// updateCacheUsage updates the metadata for the service cache entry to reflect
// that the service was reused. In particular, this updates the timestamp used
// to support aging LRU cache entries, and the count of usages of this service.
func updateCacheUsage(endpoint string) {
	if cachedItem, ok := ServiceCache[endpoint]; ok {
		cachedItem.Age = time.Now()
		cachedItem.Count++
	}
}

func updateCachedServicePackages(sessionID int, endpoint string, symbolTable *symbols.SymbolTable) {
	serviceCacheMutex.Lock()
	defer serviceCacheMutex.Unlock()

	if cachedItem, ok := ServiceCache[endpoint]; ok && cachedItem.s == nil {
		cachedItem.s = symbols.NewRootSymbolTable("packages for " + endpoint)
		count := cachedItem.s.GetPackages(symbolTable)

		ui.Log(ui.ServicesLogger, "[%d] Saved %d package definitions for %s", sessionID, count, endpoint)
	}
}

// getCachedService gets a service by endpoint name. This will either be retrieved from the
// cache, or read from disk, compiled, and then added to the cache.
func getCachedService(sessionID int, endpoint string, debug bool, file string, symbolTable *symbols.SymbolTable) (serviceCode *bytecode.ByteCode, tokens *tokenizer.Tokenizer, err error) {
	// Is this endpoint already in the cache of compiled services?
	serviceCacheMutex.Lock()
	defer serviceCacheMutex.Unlock()

	if cachedItem, ok := ServiceCache[endpoint]; ok {
		serviceCode = cachedItem.b
		tokens = cachedItem.t

		updateCacheUsage(endpoint)
		ui.Log(ui.ServicesLogger, "[%d] Using cached service compilation for %s", sessionID, endpoint)

		if debug {
			ui.Log(ui.ServicesLogger, "[%d] Debug mode enabled for this endpoints", sessionID)
		}

		if count := symbolTable.GetPackages(cachedItem.s); count > 0 {
			ui.Log(ui.ServicesLogger, "[%d] Loaded %d package definitions from cached symbols", sessionID, count)
		}
	} else {
		serviceCode, tokens, err = compileAndCacheService(sessionID, endpoint, file, symbolTable)
		// If it compiled successfully and we are caching, then put it in the cache. If we
		// are in debug mode, then we store the associated token stream; if not, then no tokens
		// are stored.
		if err == nil && MaxCachedEntries > 0 {
			var cachedTokens *tokenizer.Tokenizer

			if debug {
				cachedTokens = tokens
			}

			addToCache(sessionID, endpoint, serviceCode, cachedTokens)
		}
	}

	return
}
