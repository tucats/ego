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

func updateCacheUsage(endpoint string) {
	if cachedItem, ok := ServiceCache[endpoint]; ok {

		cachedItem.Age = time.Now()
		cachedItem.Count++
		ServiceCache[endpoint] = cachedItem
	}
}

// getCachedService gets a service by endpoint name. This will either be retrieved from the
// cache, or read from disk, compiled, and then added to the cache.
func getCachedService(sessionID int32, endpoint string, symbolTable *symbols.SymbolTable) (serviceCode *bytecode.ByteCode, tokens *tokenizer.Tokenizer, compilerInstance *compiler.Compiler, err error) {
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
		serviceCode, tokens, compilerInstance, err = compileAndCacheService(sessionID, endpoint, symbolTable)
		// If it compiled successfully and we are caching, then put it in the cache.
		if err == nil && MaxCachedEntries > 0 {
			addToCache(sessionID, endpoint, compilerInstance, serviceCode, tokens)
		}
	}

	return
}
