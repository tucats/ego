package uuid

import (
	"github.com/tucats/ego/data"
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

var UUIDPackage = data.NewPackageFromMap("uuid", map[string]interface{}{
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
