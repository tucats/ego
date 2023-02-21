package services

import (
	"strconv"
	"sync"
	"time"

	"github.com/tucats/ego/app-cli/settings"
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
