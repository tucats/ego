package i18n

import (
	"github.com/tucats/ego/data"
)

var I18nPackage = data.NewPackageFromMap("i18n", map[string]interface{}{
	"Language": data.Function{
		Declaration: &data.Declaration{
			Name:    "Language",
			Returns: []*data.Type{data.StringType},
		},
		Value: language,
	},
	"T": data.Function{
		Declaration: &data.Declaration{
			Name: "T",
			Parameters: []data.Parameter{
				{
					Name: "key",
					Type: data.StringType,
				},
				{
					Name: "parameters",
					Type: data.MapType(data.StringType, data.InterfaceType),
				},
				{
					Name: "language",
					Type: data.StringType,
				},
			},
			ArgCount: data.Range{1, 3},
			Returns:  []*data.Type{data.StringType},
		},
		Value: translation,
	},
})
