package runtime

import (
	goRuntime "runtime"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/runtime/time"
)

var FrameType = data.TypeDefinition("Frame",
	data.StructureType().
		DefineField("Module", data.StringType).
		DefineField("Table", data.StringType).
		DefineField("Line", data.IntType))

var RuntimePackage = data.NewPackageFromMap("runtime", map[string]any{
	"Frame":  FrameType,
	"GOOS":   data.Constant(goRuntime.GOOS),
	"GOARCH": data.Constant(goRuntime.GOARCH),

	// OS_* constants report facts about the host machine, captured once at
	// startup (see osInfo/captureHostInfo in host.go); MemoryAvailable()
	// below is their dynamic counterpart.
	"OS_KERNEL_VERSION":   data.Constant(osInfo.KernelVersion),
	"OS_PLATFORM":         data.Constant(osInfo.Platform),
	"OS_PLATFORM_VERSION": data.Constant(osInfo.PlatformVersion),
	"OS_CPUCORES":         data.Constant(osInfo.CPUCores),
	"OS_MEMORY":           data.Constant(osInfo.Memory),

	"MemoryAvailable": data.Function{
		Declaration: &data.Declaration{
			Name:    "MemoryAvailable",
			Returns: []*data.Type{data.IntType},
		},
		Value: memoryAvailable,
	},
	"Buildtime": data.Function{
		Declaration: &data.Declaration{
			Name:    "Buildtime",
			Returns: []*data.Type{time.TimeType, data.ErrorType},
		},
		Value: buildTime,
	},
	"Types": data.Function{
		Declaration: &data.Declaration{
			Name:    "Types",
			Returns: []*data.Type{data.StringType},
		},
		Value: types,
	},

	"Mode": data.Function{
		Declaration: &data.Declaration{
			Name:    "Mode",
			Returns: []*data.Type{data.StringType},
		},
		Value: mode,
	},
	"Ego": data.Function{
		Declaration: &data.Declaration{
			Name:    "Ego",
			Returns: []*data.Type{data.StringType},
		},
		Value: egoVersion,
	},
	"Frames": data.Function{
		Declaration: &data.Declaration{
			Name: "Frames",
			Parameters: []data.Parameter{
				{
					Name: "count",
					Type: data.IntType,
				},
			},
			Returns: []*data.Type{data.ArrayType(FrameType)},
		},
		Sandboxed: true,
		Context:   true,
		Value:     frames,
	},
	"GC": data.Function{
		Declaration: &data.Declaration{
			Name: "GC",
		},
		IsNative:  true,
		Sandboxed: true,
		Value:     goRuntime.GC,
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
		Context:   true,
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
