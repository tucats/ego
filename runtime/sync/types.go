package sync

import (
	"sync"

	"github.com/tucats/ego/data"
)

var SyncWaitGroupType = data.TypeDefinition("WaitGroup", data.StructureType()).
	SetNativeName("sync.WaitGroup").
	SetPackage("sync").
	SetNew(func() interface{} {
		return new(sync.WaitGroup)
	}).
	DefineNativeFunction("Add",
		&data.Declaration{
			Name: "Add",
			Type: data.OwnType,
			Parameters: []data.Parameter{
				{
					Name: "count",
					Type: data.IntType,
				},
			},
		}, nil).
	DefineNativeFunction("Done",
		&data.Declaration{
			Name: "Done",
			Type: data.OwnType,
		}, nil).
	DefineNativeFunction("Wait",
		&data.Declaration{
			Name: "Wait",
			Type: data.OwnType,
		}, nil)

var SyncMutexType = data.TypeDefinition("Mutex", data.StructureType()).
	SetNativeName("sync.Mutex").
	SetPackage("sync").
	SetNew(func() interface{} {
		return new(sync.Mutex)
	}).
	DefineNativeFunction("Lock", &data.Declaration{
		Name: "Lock",
		Type: data.OwnType,
	}, nil).
	DefineNativeFunction("Unlock", &data.Declaration{
		Name: "Unlock",
		Type: data.OwnType,
	}, nil).
	DefineNativeFunction("TryLock", &data.Declaration{
		Name:    "TryLock",
		Type:    data.OwnType,
		Returns: []*data.Type{data.BoolType},
	}, nil)

var SyncRWMutexType = data.TypeDefinition("RWMutex", data.StructureType()).
	SetNativeName("sync.RWMutex").
	SetPackage("sync").
	SetNew(func() interface{} {
		return new(sync.RWMutex)
	}).
	DefineNativeFunction("Lock",
		&data.Declaration{
			Name: "Lock",
			Type: data.OwnType,
		}, nil).
	DefineNativeFunction("RLock",
		&data.Declaration{
			Type: data.OwnType,
			Name: "RLock",
		}, nil).
	DefineNativeFunction("Unlock",
		&data.Declaration{
			Name: "Unlock",
			Type: data.OwnType,
		}, nil).
	DefineNativeFunction("RUnlock",
		&data.Declaration{
			Name: "RUnlock",
			Type: data.OwnType,
		}, nil).
	DefineNativeFunction("TryLock",
		&data.
			Declaration{
			Name:    "TryLock",
			Type:    data.OwnType,
			Returns: []*data.Type{data.BoolType},
		}, nil).
	DefineNativeFunction("RTryLock",
		&data.Declaration{
			Name:    "RTryLock",
			Type:    data.OwnType,
			Returns: []*data.Type{data.BoolType},
		}, nil)

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
var SyncPackage = data.NewPackageFromMap("sync", map[string]interface{}{
	"WaitGroup": SyncWaitGroupType,
	"Mutex":     SyncMutexType,
})
