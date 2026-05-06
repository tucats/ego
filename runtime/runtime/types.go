package runtime

import (
	goRuntime "runtime"

	"github.com/tucats/ego/data"
)

var RuntimePackage = data.NewPackageFromMap("runtime", map[string]any{
	"GOOS":   data.Constant(goRuntime.GOOS),
	"GOARCH": data.Constant(goRuntime.GOARCH),
	"GC": data.Function{
		Declaration: &data.Declaration{
			Name: "GC",
		},
		IsNative: true,
		Value:    goRuntime.GC,
	},
	"GOMAXPROCS": data.Function{
		Declaration: &data.Declaration{
			Name: "GOMAXPROCS",
			Parameters: []data.Parameter{
				{
					Name: "n",
					Type: data.IntType,
				},
			},
			Returns: []*data.Type{data.IntType},
		},
		IsNative:  true,
		Sandboxed: true,
		Value:     goRuntime.GOMAXPROCS,
	},
	"NumCPU": data.Function{
		Declaration: &data.Declaration{
			Name:    "NumCPU",
			Returns: []*data.Type{data.IntType},
		},
		IsNative: true,
		Value:    goRuntime.NumCPU,
	},
	"Stack": data.Function{
		Declaration: &data.Declaration{
			Name: "Stack",
			Parameters: []data.Parameter{
				{
					Name: "buf",
					Type: data.ArrayType(data.ByteType),
				},
				{
					Name: "all",
					Type: data.BoolType,
				},
			},
			Returns: []*data.Type{data.IntType},
		},
		Sandboxed: true,
		Value:     stack,
	},
	"Version": data.Function{
		Declaration: &data.Declaration{
			Name:    "Version",
			Returns: []*data.Type{data.StringType},
		},
		IsNative: true,
		Value:    goRuntime.Version,
	},
})
