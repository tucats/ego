package data

import (
	"fmt"
	"sort"
	"sync"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
)

// This describes the items in a package. Each item could be an arbitrary object
// (function, data type, etc).
type Package struct {
	name     string
	ID       string
	imported bool
	builtins bool
	items    map[string]interface{}
}

// This mutex protects ALL packages. This serializes package operations across all threads. This
// should only materially affect parallel compilation operations, which will become slightly more
// synchronous.
var packageLock sync.RWMutex

// NewPackage creates a new, empty package definition.
func NewPackage(name string) *Package {
	pkg := Package{
		name:  name,
		ID:    uuid.New().String(),
		items: map[string]interface{}{},
	}

	return &pkg
}

// NewPackageFromMap creates a new package, and then populates it using the provided map.  If the map
// is a nil value, then an empty package definition is created.
func NewPackageFromMap(name string, items map[string]interface{}) *Package {
	if items == nil {
		items = map[string]interface{}{}
	}

	pkg := Package{
		name:  name,
		ID:    uuid.New().String(),
		items: items,
	}

	return &pkg
}

func (p *Package) SetBuiltins(f bool) *Package {
	p.builtins = f

	return p
}

func (p *Package) Builtins() bool {
	return p.builtins
}

func (p *Package) SetImported(f bool) *Package {
	p.imported = f

	return p
}

func (p *Package) HasTypes() bool {
	for _, v := range p.items {
		if t, ok := v.(*Type); ok {
			if hasCapitalizedName(t.name) {
				return true
			}
		}
	}

	return false
}

func (p *Package) Constants() bool {
	for _, v := range p.items {
		if _, ok := v.(*Type); ok {
			return true
		}
	}

	return false
}

func (p *Package) HasImportedSource() bool {
	return p.imported
}

// IsEmpty reports if a package is empty. This could be due to a null pointer, uninitialized
// internal hash map, or an empty hash map.
func (p *Package) IsEmpty() bool {
	if p == nil {
		return true
	}

	if p.items == nil {
		return true
	}

	return len(p.items) > 0
}

// String formats the package as a string value, to support "%v" operations.
func (p *Package) String() string {
	return Format(p)
}

// Delete removes a package from the list. It is not an error if the package does not
// have a hash map, or the value is not in the hash map.
func (p *Package) Delete(name string) {
	packageLock.Lock()
	defer packageLock.Unlock()

	if p.items != nil {
		delete(p.items, name)
	}
}

// Keys provides a list of keys for the package as an array of strings. The array will
// be empty if the package pointer is null, the hash map is uninitialized, or the hash
// map is empty.
func (p *Package) Keys() []string {
	packageLock.RLock()
	defer packageLock.RUnlock()

	keys := make([]string, 0)

	if p != nil && p.items != nil {
		for k := range p.items {
			keys = append(keys, k)
		}

		sort.Strings(keys)
	}

	return keys
}

// Set sets a given value in the package. If the hash map was not yet initialized,
// it is created now before setting the value.
func (p *Package) Set(key string, value interface{}) {
	packageLock.Lock()
	defer packageLock.Unlock()

	if p.items == nil {
		p.items = map[string]interface{}{}
	}

	// If we're doing symbol tracing, indicate what we're doing (set vs. update) for the
	// given package, key, and value.
	if ui.IsActive(ui.SymbolLogger) {
		v := Format(value)
		action := "set"

		if _, ok := p.items[key]; ok {
			action = "update"
		}

		ui.Log(ui.SymbolLogger, fmt.Sprintf(" for package %s, %s %s to %#v", p.name, action, key, v))
	}

	p.items[key] = value
}

// Get retrieves a value from the package structure by name. It returns the value and
// a boolean value indicating if it was found. The flag is true if the package has been
// initialized, the hash map is initialized, and the named value is found in the hashmap.
func (p *Package) Get(key string) (interface{}, bool) {
	packageLock.RLock()
	defer packageLock.RUnlock()

	if p.items == nil {
		return nil, false
	}

	value, found := p.items[key]

	return value, found
}

// Merge adds any entries from a package to the current package that do not already
// exist.
func (p *Package) Merge(source Package) {
	keys := source.Keys()
	for _, key := range keys {
		if _, found := p.Get(key); !found {
			value, _ := source.Get(key)
			p.Set(key, value)
			ui.Log(ui.CompilerLogger, "... merging key %s from existing package", key)
		}
	}
}

func (p *Package) Name() string {
	return p.name
}
