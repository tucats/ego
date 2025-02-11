package filepath

import (
	"path/filepath"

	"github.com/tucats/ego/data"
)

var FilepathPackage = data.NewPackageFromMap("filepath", map[string]interface{}{
	"Abs": data.Function{
		Declaration: &data.Declaration{
			Name: "Abs",
			Parameters: []data.Parameter{
				{
					Name: "partialPath",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.StringType, data.ErrorType},
		},
		Value:    filepath.Abs,
		IsNative: true,
	},
	"Base": data.Function{
		Declaration: &data.Declaration{
			Name: "Base",
			Parameters: []data.Parameter{
				{
					Name: "path",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value:    filepath.Base,
		IsNative: true,
	},
	"Clean": data.Function{
		Declaration: &data.Declaration{
			Name: "Clean",
			Parameters: []data.Parameter{
				{
					Name: "path",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value:    filepath.Clean,
		IsNative: true,
	},
	"Dir": data.Function{
		Declaration: &data.Declaration{
			Name: "Dir",
			Parameters: []data.Parameter{
				{
					Name: "path",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value:    filepath.Dir,
		IsNative: true,
	},
	"Ext": data.Function{
		Declaration: &data.Declaration{
			Name: "Ext",
			Parameters: []data.Parameter{
				{
					Name: "path",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value:    filepath.Ext,
		IsNative: true,
	},
	"Join": data.Function{
		Declaration: &data.Declaration{
			Name: "Join",
			Parameters: []data.Parameter{
				{
					Name: "elements",
					Type: data.StringType,
				},
			},
			Variadic: true,
			Returns:  []*data.Type{data.StringType},
		},
		Value:    filepath.Join,
		IsNative: true,
	},
})
