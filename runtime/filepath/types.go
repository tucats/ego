package filepath

import (
	"path/filepath"

	"github.com/tucats/ego/data"
)

var FilepathPackage = data.NewPackageFromMap("filepath", map[string]any{
	"Abs": data.Function{
		Declaration: &data.Declaration{
			Name: "Abs",
			Parameters: []data.Parameter{
				{
					Name:      "partialPath",
					Type:      data.StringType,
					Sandboxed: true,
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
					Name:      "path",
					Type:      data.StringType,
					Sandboxed: true,
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
					Name:      "path",
					Type:      data.StringType,
					Sandboxed: true,
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
					Name:      "path",
					Type:      data.StringType,
					Sandboxed: true,
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
					Name:      "path",
					Type:      data.StringType,
					Sandboxed: true,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value:    filepath.Ext,
		IsNative: true,
	},
	// Join accepts a variadic list of path elements. The sandbox prefix is applied
	// only to the first element (index 0) because the bytecode engine's parameter
	// sandbox check stops at len(Parameters). Remaining variadic elements (index ≥ 1)
	// are not individually prefixed — a limitation in callNative.go:101.
	"Join": data.Function{
		Declaration: &data.Declaration{
			Name: "Join",
			Parameters: []data.Parameter{
				{
					Name:      "elements",
					Type:      data.StringType,
					Sandboxed: true,
				},
			},
			Variadic: true,
			Returns:  []*data.Type{data.StringType},
		},
		Value:    filepath.Join,
		IsNative: true,
	},
})
