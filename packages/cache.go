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

func Save(pkg *data.Package) *data.Package {
	cacheLock.Lock()
	defer cacheLock.Unlock()

	cache[pkg.Path] = pkg

	return pkg
}

func Delete(path string) *data.Package {
	cacheLock.Lock()
	defer cacheLock.Unlock()

	pkg := cache[path]

	delete(cache, path)

	return pkg
}
