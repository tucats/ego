package sync

import (
	"sync"

	"github.com/tucats/ego/data"
)

var SyncWaitGroupType = data.TypeDefinition("WaitGroup", data.StructureType()).
	SetNativeName("sync.WaitGroup").
	SetPackage("sync").
	SetNew(func() any {
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
		}, nil).FixSelfReferences()

var SyncMutexType = data.TypeDefinition("Mutex", data.StructureType()).
	SetNativeName("sync.Mutex").
	SetPackage("sync").
	SetNew(func() any {
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
	}, nil).FixSelfReferences()

var SyncRWMutexType = data.TypeDefinition("RWMutex", data.StructureType()).
	SetNativeName("sync.RWMutex").
	SetPackage("sync").
	SetNew(func() any {
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
		}, nil).FixSelfReferences()

var SyncPackage = data.NewPackageFromMap("sync", map[string]any{
	"WaitGroup": SyncWaitGroupType,
	"Mutex":     SyncMutexType,
})
