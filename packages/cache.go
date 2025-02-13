package packages

import (
	"sync"

	"github.com/tucats/ego/data"
)

var cache = make(map[string]*data.Package)
var cacheLock sync.Mutex

// Get returns the package by the path given on the import statement.
func Get(path string) *data.Package {
	cacheLock.Lock()
	defer cacheLock.Unlock()

	return cache[path]
}

// GetByName returns the package by the given package name in the
// package statements (which is distinct form the package by the path
// given on the import statement).
func GetByName(name string) *data.Package {
	for _, pkg := range cache {
		if pkg.Name == name {
			return pkg
		}
	}

	return nil
}

// Save stores/updates the provided package in the cache.
func Save(pkg *data.Package) *data.Package {
	if pkg == nil {
		return nil
	}

	cacheLock.Lock()
	defer cacheLock.Unlock()

	if pkg.Path == "" {
		pkg.Path = pkg.Name
	}

	cache[pkg.Path] = pkg

	return pkg
}

// Delete removes a package from the cache.
func Delete(path string) *data.Package {
	cacheLock.Lock()
	defer cacheLock.Unlock()

	pkg := cache[path]

	delete(cache, path)

	return pkg
}
