package data

import (
	"sort"
	"sync"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
)

// This describes the items in a package. A package consists of a map of items,
// which may includes constant definitions, type definitions, function bodies,
// or receiver function bodies. It also includes metadata regarding whether it
// has import source, or includes Go-native builtins.
type Package struct {
	// Name is the name of the package. This must be a valid Ego identifier string.
	Name string

	// This is the path that defines the package. For most if not all builtin packages,
	// this is the same as the Name. For user-created packages, it will be the file
	// system path to the package source files.
	Path string

	// ID is the UUID of this package. Each package is given a unique ID on creation,
	// to assist in debugging package operations.
	ID string

	// Source indicates that this package includes constants, types, or functions
	// that were read from source files as part of an "import" operations.
	Source bool

	// Builtins is true if the package includes one ore more native (Go) functions.
	Builtins bool

	// Types is true if the package includes one ore more type definitions.
	Types bool

	// Constants is true if the package includes one or more const declarations.
	Constants bool

	// Items contains map of named constants, types, and functions for this package.
	items map[string]any
}

// This mutex protects ALL packages. This serializes package operations across all threads. This
// should only materially affect parallel compilation operations, which will become slightly more
// synchronous.
var packageLock sync.RWMutex

// NewPackage creates a new, empty package definition. The supplied name must be a valid Ego
// identifier name. The package is assigned a unique UUID at the time of creation that never
// changes for the life of this package object.
func NewPackage(name, path string) *Package {
	pkg := Package{
		Name:  name,
		Path:  path,
		ID:    uuid.New().String(),
		items: map[string]any{},
	}

	return &pkg
}

// NewPackageFromMap creates a new package, and then populates it using the provided map.  If the map
// is a nil value, then an empty package definition is created.
func NewPackageFromMap(name string, items map[string]any) *Package {
	if items == nil {
		items = map[string]any{}
	}

	// Build a package.
	pkg := &Package{
		Name:  name,
		Path:  name,
		ID:    uuid.New().String(),
		items: items,
	}

	// Sanity check. For any function defined in the map, make sure it's key matches the function
	// name.
	for k, v := range items {
		if fn, ok := v.(Function); ok {
			if fn.Declaration.Name != k {
				ui.Log(ui.InternalLogger, "pkg.map.function.mismatch", ui.A{
					"package":  name,
					"expected": k,
					"actual":   fn.Declaration.Name,
				})
			}
		}
	}

	// Add the items from the map. If we are importing a function that is a
	// language extensions, and extensions aren't enabled, skip it.
	for _, v := range items {
		updatePackageClassIndicators(pkg, v)
	}

	return pkg
}

func (p *Package) Initialize(fn func(p *Package)) *Package {
	// Serialize the operation.
	packageLock.Lock()
	defer packageLock.Unlock()

	fn(p)

	return p
}

// SetBuiltins sets the imported flag for the package. This flag indicates
// that the package includes type, constants, or functions that came from
// a source file that was read as part of an "import" statement.
//
// The function returns the same *Package it received, so this can be
// chained with other "set" functions.
func (p *Package) SetImported(f bool) *Package {
	// Serialize the operation.
	packageLock.Lock()
	defer packageLock.Unlock()

	if p == nil {
		ui.Log(ui.InternalLogger, "runtime.pkg.nil.write", nil)

		return nil
	}

	p.Source = f

	return p
}

// HasTypes returns true if the package contains one ore more Type
// declarations.
func (p *Package) HasTypes() bool {
	// Serialize the operation.
	packageLock.Lock()
	defer packageLock.Unlock()

	if p == nil {
		ui.Log(ui.InternalLogger, "runtime.pkg.nil.read", nil)

		return false
	}

	for _, v := range p.items {
		if t, ok := v.(*Type); ok {
			if hasCapitalizedName(t.name) {
				return true
			}
		}
	}

	return false
}

// IsEmpty reports if a package is empty. This could be due to a null pointer, uninitialized
// internal hash map, or an empty hash map.
func (p *Package) IsEmpty() bool {
	// Serialize the operation.
	packageLock.Lock()
	defer packageLock.Unlock()

	if p == nil {
		return true
	}

	if p.items == nil {
		return true
	}

	return len(p.items) == 0
}

// String formats the package data as a string value, to support "%v" operations.
func (p *Package) String() string {
	// Serialize the operation.
	packageLock.Lock()
	defer packageLock.Unlock()

	return Format(p)
}

// Delete removes an item from the package. It is not an error if the package did
// not contain the named item. This operation is thread-safe.
func (p *Package) Delete(name string) {
	if p == nil {
		ui.Log(ui.InternalLogger, "runtime.pkg.nil.write", nil)

		return
	}

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
	if p == nil {
		ui.Log(ui.InternalLogger, "runtime.pkg.nil.read", nil)

		return nil
	}

	packageLock.RLock()
	defer packageLock.RUnlock()

	keys := make([]string, 0)

	if p.items != nil {
		for k := range p.items {
			keys = append(keys, k)
		}

		sort.Strings(keys)
	}

	return keys
}

// Set sets a given value in the package. If the hash map was not yet initialized,
// it is created now before setting the value.
func (p *Package) Set(key string, value any) {
	if p == nil {
		ui.Log(ui.InternalLogger, "runtime.pkg.nil.write", nil)

		return
	}

	packageLock.Lock()
	defer packageLock.Unlock()

	if p.items == nil {
		p.items = map[string]any{}
	}

	// If we're doing symbol tracing, indicate what we're doing (set vs. update) for the
	// given package, key, and value.
	if ui.IsActive(ui.SymbolLogger) {
		v := Format(value)
		action := "set"

		if _, ok := p.items[key]; ok {
			action = "update"
		}

		ui.Log(ui.SymbolLogger, "symbols.pkg.set", ui.A{
			"package": p.Name,
			"name":    p.Name,
			"action":  action,
			"key":     key,
			"value":   v})
	}

	updatePackageClassIndicators(p, value)

	p.items[key] = value
}

// Get retrieves a value from the package structure by name. It returns the value and
// a boolean value indicating if it was found. The flag is true if the package has been
// initialized, the hash map is initialized, and the named value is found in the hashmap.
func (p *Package) Get(key string) (any, bool) {
	if p == nil {
		ui.Log(ui.InternalLogger, "runtime.pkg.nil.read", nil)

		return nil, false
	}

	packageLock.RLock()
	defer packageLock.RUnlock()

	if p.items == nil {
		return nil, false
	}

	value, found := p.items[key]

	return value, found
}

// updatePackageClassIndicators updates the various boolean flags in the package
// based on the type of the value. These flags track whether there are Types,
// Constants, Builtins, or Imports in this package.
func updatePackageClassIndicators(pkg *Package, v any) {
	if pkg == nil {
		ui.Log(ui.InternalLogger, "runtime.pkg.nil.write", nil)

		return
	}

	if _, ok := v.(*Type); ok {
		pkg.Types = true
		pkg.Builtins = true
	} else if _, ok := v.(Immutable); ok {
		pkg.Constants = true
		pkg.Builtins = true
	} else if _, ok := v.(Function); ok {
		pkg.Builtins = true
	}
}
