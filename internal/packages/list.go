package packages

import "sort"

// Return a list of the package paths currently in the cache.
// Native packages are listed first in alphabetical order,
// followed by user-defined packages in alphabetical order.
func List() []string {
	var native, user []string

	cacheLock.Lock()
	defer cacheLock.Unlock()

	for path, pkg := range cache {
		if pkg.Name == pkg.Path {
			native = append(native, path)
		} else {
			user = append(user, path)
		}
	}

	sort.Strings(native)
	sort.Strings(user)

	paths := make([]string, 0, len(cache))

	for _, path := range native {
		paths = append(paths, path)
	}

	for _, path := range user {
		paths = append(paths, path)
	}

	return paths
}
