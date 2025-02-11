package base64

import (
	"github.com/tucats/ego/data"
)

var Base64Package = data.NewPackageFromMap("base64", map[string]interface{}{
	"Decode": data.Function{
		Declaration: &data.Declaration{
			Name: "Decode",
			Parameters: []data.Parameter{
				{
					Name: "data",
					Type: data.StringType,
				},
			},
			Returns:  []*data.Type{data.StringType},
			ArgCount: data.Range{1, 1},
		},
		Value: decode,
	},
	"Encode": data.Function{
		Declaration: &data.Declaration{
			Name: "Encode",
			Parameters: []data.Parameter{
				{
					Name: "data",
					Type: data.StringType,
				},
			},
			Returns:  []*data.Type{data.StringType},
			ArgCount: data.Range{1, 1},
		},
		Value: encode,
	},
})
