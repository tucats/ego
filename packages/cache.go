package packages

import (
	"sync"

	"github.com/tucats/ego/data"
)

var cache = make(map[string]*data.Package)
var cacheLock sync.Mutex

func Get(path string) *data.Package {
	cacheLock.Lock()
	defer cacheLock.Unlock()

	return cache[path]
}

func Save(pkg *data.Package) {
	cacheLock.Lock()
	defer cacheLock.Unlock()

	cache[pkg.Path] = pkg
}

func Delete(path string) {
	cacheLock.Lock()
	defer cacheLock.Unlock()

	delete(cache, path)
}
