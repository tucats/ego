package profile

import (
	"github.com/tucats/ego/data"
)

var ProfilePackage = data.NewPackageFromMap("profile", map[string]interface{}{
	"Delete": data.Function{
		Declaration: &data.Declaration{
			Name: "Delete",
			Parameters: []data.Parameter{
				{
					Name: "key",
					Type: data.StringType,
				},
			},
		},
		Value: deleteKey,
	},
	"Get": data.Function{
		Declaration: &data.Declaration{
			Name: "Get",
			Parameters: []data.Parameter{
				{
					Name: "key",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value: getKey,
	},
	"Keys": data.Function{
		Declaration: &data.Declaration{
			Name:    "Keys",
			Returns: []*data.Type{data.ArrayType(data.StringType)},
		},
		Value: getKeys,
	},
	"Config": data.Function{
		Declaration: &data.Declaration{
			Name:    "Config",
			Returns: []*data.Type{data.MapType(data.StringType, data.StringType)},
		},
		Value: getConfig,
	},
	"Set": data.Function{
		Declaration: &data.Declaration{
			Name: "Set",
			Parameters: []data.Parameter{
				{
					Name: "key",
					Type: data.StringType,
				},
				{
					Name: "value",
					Type: data.StringType,
				},
			},
			Scope: true,
		},
		Value: setKey,
	},
})
