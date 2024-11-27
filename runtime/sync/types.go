package sync

import (
	"sync"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

var waitGroupType *data.Type
var mutextType *data.Type
var rwMutexType *data.Type

var initLock sync.Mutex

// Initialize creates the "sync" package and defines it's functions and the default
// structure definition. This is serialized so it will only be done once, no matter
// how many times called.
//
// Note that "sync" is an example of a package that is completely native. That is,
// there are no shim routines to support it; the metadata defined in the types is
// all that is used to create a new instance of an object of the given types, or to
// make calls to the native functions.
//
// Because these objects have an instnace of the noCopy field in them, they are
// not allowed to be copied or they would break their functionality. As such, the
// new instances are always pointers to new objects. This means that the SetNew()
// method defines the function that calls the native Go new() function, and as such
// all code that validates types, etc. will assume the underlying value has an extra
// pointer dereference.
func Initialize(s *symbols.SymbolTable) {
	initLock.Lock()
	defer initLock.Unlock()

	if mutextType == nil {
		mutextType = data.TypeDefinition("Mutex", data.StructureType()).
			SetNativeName("sync.Mutex").
			SetPackage("sync").
			SetNew(func() interface{} {
				return new(sync.Mutex)
			})

		mutextType.DefineNativeFunction("Lock",
			&data.Declaration{
				Name: "Lock",
				Type: mutextType,
			}, nil)

		mutextType.DefineNativeFunction("Unlock",
			&data.Declaration{
				Name: "Unlock",
				Type: mutextType,
			}, nil)

		mutextType.DefineNativeFunction("TryLock",
			&data.Declaration{
				Name:    "TryLock",
				Type:    data.BoolType,
				Returns: []*data.Type{data.BoolType},
			}, nil)
	}

	if rwMutexType == nil {
		rwMutexType = data.TypeDefinition("RWMutex", data.StructureType()).
			SetNativeName("sync.RWMutex").
			SetPackage("sync").
			SetNew(func() interface{} {
				return new(sync.RWMutex)
			})

		rwMutexType.DefineNativeFunction("Lock",
			&data.Declaration{
				Name: "Lock",
				Type: rwMutexType,
			}, nil)

		rwMutexType.DefineNativeFunction("RLock",
			&data.Declaration{
				Name: "RLock",
				Type: rwMutexType,
			}, nil)

		rwMutexType.DefineNativeFunction("Unlock",
			&data.Declaration{
				Name: "Unlock",
				Type: rwMutexType,
			}, nil)

		rwMutexType.DefineNativeFunction("RUnlock",
			&data.Declaration{
				Name: "RUnlock",
				Type: rwMutexType,
			}, nil)

		mutextType.DefineNativeFunction("TryLock",
			&data.Declaration{
				Name:    "TryLock",
				Type:    data.BoolType,
				Returns: []*data.Type{data.BoolType},
			}, nil)

		mutextType.DefineNativeFunction("RTryLock",
			&data.Declaration{
				Name:    "RTryLock",
				Type:    data.BoolType,
				Returns: []*data.Type{data.BoolType},
			}, nil)
	}

	if waitGroupType == nil {
		waitGroupType = data.TypeDefinition("WaitGroup", data.StructureType()).
			SetNativeName("sync.WaitGroup").
			SetPackage("sync").
			SetNew(func() interface{} {
				return new(sync.WaitGroup)
			})

		waitGroupType.DefineNativeFunction("Add",
			&data.Declaration{
				Name: "Add",
				Type: waitGroupType,
				Parameters: []data.Parameter{
					{
						Name: "count",
						Type: data.IntType,
					},
				},
			}, nil)

		waitGroupType.DefineNativeFunction("Done",
			&data.Declaration{
				Name: "Done",
				Type: waitGroupType,
			}, nil)

		waitGroupType.DefineNativeFunction("Wait",
			&data.Declaration{
				Name: "Wait",
				Type: waitGroupType,
			}, nil)
	}

	if _, found := s.Root().Get("sync"); !found {
		newpkg := data.NewPackageFromMap("sync", map[string]interface{}{
			"WaitGroup": waitGroupType,
			"Mutex":     mutextType,
		})

		pkg, _ := bytecode.GetPackage(newpkg.Name)
		pkg.Merge(newpkg)
		s.Root().SetAlways(newpkg.Name, newpkg)
	}
}
