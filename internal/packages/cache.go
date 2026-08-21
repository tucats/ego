package packages

import (
	"sync"

	"github.com/tucats/ego/internal/language/data"
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
// package statements (which is distinct from the package by the path
// given on the import statement).
//
// Fix: unlike every other function in this file, this one used to range
// over the cache map without holding cacheLock at all. The server calls this
// (via bytecode.GetPackage) while handling requests concurrently, and Save()
// below writes to the very same map from a different goroutine whenever a
// package is imported for the first time. Save() taking the lock does not
// help on its own -- *every* accessor of a Go map needs to agree on the same
// lock, or an unprotected reader here can still observe the map mid-write
// (for example, while it is being resized internally to hold a new entry),
// which is undefined behavior and can crash or hang the process.
func GetByName(name string) *data.Package {
	cacheLock.Lock()
	defer cacheLock.Unlock()

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
