package services

import (
	"strconv"
	"sync"
	"time"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
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
	c     *compiler.Compiler
	b     *bytecode.ByteCode
	t     *tokenizer.Tokenizer
	s     *symbols.SymbolTable
	Count int
}

// ServiceCache is a map that contains compilation data for previously-
// compiled service handlers written in the Ego language.
var ServiceCache = map[string]CachedCompilationUnit{}
var serviceCacheMutex sync.Mutex

// MaxCachedEntries is the maximum number of items allowed in the service
// cache before items start to be aged out (oldest first).
// @tomcole there is currently a bug where multiple uses of the same cached
// server at the same time fail to find package symbols correctly. As such
// the cache is currently disabled.
var MaxCachedEntries = 0

// setupServiceCache ensures that the service cache is configured.
func setupServiceCache() {
	serviceCacheMutex.Lock()
	if MaxCachedEntries < 0 {
		txt := settings.Get(defs.MaxCacheSizeSetting)
		MaxCachedEntries, _ = strconv.Atoi(txt)
	}
	serviceCacheMutex.Unlock()
}

// Update the cache entry for a given endpoint with the supplied compiler, bytecode, and tokens. If necessary,
// age out the oldest cached item (based on last time-of-access) from the cache to keep it within the maximum
// cache size.
func addToCache(session int, endpoint string, comp *compiler.Compiler, code *bytecode.ByteCode, tokens *tokenizer.Tokenizer) {
	ui.Log(ui.InfoLogger, "[%d] Caching compilation unit for %s", session, endpoint)

	ServiceCache[endpoint] = CachedCompilationUnit{
		Age:   time.Now(),
		c:     comp,
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
		ui.Log(ui.InfoLogger, "[%d] Endpoint %s aged out of cache", session, key)
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
		ServiceCache[endpoint] = cachedItem
	}
}

func updateCachedServicePackages(sessionID int, endpoint string, symbolTable *symbols.SymbolTable) {
	serviceCacheMutex.Lock()
	defer serviceCacheMutex.Unlock()

	if cachedItem, ok := ServiceCache[endpoint]; ok && cachedItem.s == nil {
		cachedItem.s = symbols.NewRootSymbolTable("packages for " + endpoint)
		count := cachedItem.s.GetPackages(symbolTable)
		ServiceCache[endpoint] = cachedItem

		ui.Log(ui.InfoLogger, "[%d] Caching %d package definitions for %s", sessionID, count, endpoint)
	}
}

// getCachedService gets a service by endpoint name. This will either be retrieved from the
// cache, or read from disk, compiled, and then added to the cache.
func getCachedService(sessionID int, endpoint, file string, symbolTable *symbols.SymbolTable) (serviceCode *bytecode.ByteCode, tokens *tokenizer.Tokenizer, compilerInstance *compiler.Compiler, err error) {
	// Is this endpoint already in the cache of compiled services?
	serviceCacheMutex.Lock()
	defer serviceCacheMutex.Unlock()

	if cachedItem, ok := ServiceCache[endpoint]; ok {
		compilerInstance = cachedItem.c.Clone(true)
		serviceCode = cachedItem.b
		tokens = cachedItem.t

		symbolTable.GetPackages(cachedItem.s)
		updateCacheUsage(endpoint)
		compilerInstance.AddPackageToSymbols(symbolTable)
		ui.Log(ui.InfoLogger, "[%d] Using cached compilation unit for %s", sessionID, endpoint)
	} else {
		serviceCode, tokens, compilerInstance, err = compileAndCacheService(sessionID, endpoint, file, symbolTable)
		// If it compiled successfully and we are caching, then put it in the cache.
		if err == nil && MaxCachedEntries > 0 {
			addToCache(sessionID, endpoint, compilerInstance, serviceCode, tokens)
		}
	}

	return
}
