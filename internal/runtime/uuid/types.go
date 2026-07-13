package uuid

import (
	"github.com/google/uuid"
	"github.com/tucats/ego/internal/language/data"
)

var UUIDTypeDef = data.NewType("UUID", data.StructKind).SetNativeName("uuid.UUID").SetPackage("uuid").
	DefineFunctions(map[string]data.Function{
		"String": {
			Declaration: &data.Declaration{
				Name:    "String",
				Type:    data.OwnType,
				Returns: []*data.Type{data.StringType},
			},
			Value: toString,
		},
		"Gibberish": {
			Declaration: &data.Declaration{
				Name:    "Gibberish",
				Type:    data.OwnType,
				Returns: []*data.Type{data.StringType},
			},
			Value: toGibberish,
		},
	}).FixSelfReferences()

// init wires up UUIDTypeDef's zero-value constructor. This can't be done as
// part of the var declaration above (a closure referencing UUIDTypeDef inside
// its own initializer is an initialization cycle); it must run after
// UUIDTypeDef is fully assigned.
//
// Without this, a bare "var x uuid.UUID" (or new(uuid.UUID)) falls back to
// Type.InstanceOf's generic StructKind case -- an empty NewStruct(t) with no
// native field at all -- so every method call (String, Gibberish) failed with
// "invalid field name for type: native value". Go's own zero value for
// uuid.UUID ([16]byte) is the perfectly valid nil UUID, so Ego's zero value
// should behave the same way.
func init() {
	UUIDTypeDef.SetNew(func() any {
		return data.NewStruct(UUIDTypeDef).SetNative(uuid.Nil)
	})
}

var UUIDPackage = data.NewPackageFromMap("uuid", map[string]any{
	"New": data.Function{
		Declaration: &data.Declaration{
			Name:    "New",
			Returns: []*data.Type{UUIDTypeDef},
		},
		Value: newUUID,
	},
	"Nil": data.Function{
		Declaration: &data.Declaration{
			Name:    "Nil",
			Returns: []*data.Type{UUIDTypeDef},
		},
		Value: nilUUID,
	},
	"Parse": data.Function{
		Declaration: &data.Declaration{
			Name: "Parse",
			Parameters: []data.Parameter{
				{
					Name: "text",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{UUIDTypeDef, data.ErrorType},
		},
		Value: parseUUID,
	},
	"UUID": UUIDTypeDef,
})
