package os

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func Initialize(s *symbols.SymbolTable) {
	newpkg := data.NewPackageFromMap("os", map[string]interface{}{
		"Args": data.Function{
			Declaration: &data.Declaration{
				Name:    "Args",
				Returns: []*data.Type{data.ArrayType(data.StringType)},
			},
			Value: args,
		},
		"Chdir": data.Function{
			Declaration: &data.Declaration{
				Name: "Chdir",
				Parameters: []data.Parameter{
					{
						Name: "path",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.ErrorType},
			},
			Value: changeDirectory,
		},
		"Chmod": data.Function{
			Declaration: &data.Declaration{
				Name: "Chmod",
				Parameters: []data.Parameter{
					{
						Name: "file",
						Type: data.StringType,
					},
					{
						Name: "mode",
						Type: data.IntType,
					},
				},
				Returns: []*data.Type{data.ErrorType},
			},
			Value: changeMode,
		},
		"Chown": data.Function{
			Declaration: &data.Declaration{
				Name: "Chown",
				Parameters: []data.Parameter{
					{
						Name: "file",
						Type: data.StringType,
					},
					{
						Name: "owner",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.ErrorType},
			},
			Value: changeOwner,
		},
		"Clearenv": data.Function{
			Declaration: &data.Declaration{
				Name: "Clearenv",
			},
			Value: clearEnv,
		},
		"Environ": data.Function{
			Declaration: &data.Declaration{
				Name:    "Environ",
				Returns: []*data.Type{data.ArrayType(data.StringType)},
			},
			Value: Environ,
		},
		"Executable": data.Function{
			Declaration: &data.Declaration{
				Name:    "Executable",
				Returns: []*data.Type{data.StringType},
			},
			Value: executable,
		},
		"Exit": data.Function{
			Declaration: &data.Declaration{
				Name: "Exit",
				Parameters: []data.Parameter{
					{
						Name: "code",
						Type: data.IntType,
					},
				},
				ArgCount: data.Range{0, 1},
			},
			Value: exit,
		},
		"Getenv": data.Function{
			Declaration: &data.Declaration{
				Name: "Getenv",
				Parameters: []data.Parameter{
					{
						Name: "name",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: getEnv,
		},
		"Hostname": data.Function{
			Declaration: &data.Declaration{
				Name:    "Hostname",
				Returns: []*data.Type{data.StringType},
			},
			Value: hostname,
		},
		"ReadFile": data.Function{
			Declaration: &data.Declaration{
				Name: "ReadFile",
				Parameters: []data.Parameter{
					{
						Name: "filename",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.ArrayType(data.ByteType), data.ErrorType},
			},
			Value: readFile,
		},
		"Remove": data.Function{
			Declaration: &data.Declaration{
				Name: "Remove",
				Parameters: []data.Parameter{
					{
						Name: "filename",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.ErrorType},
			},
			Value: removeFile,
		},
		"Writefile": data.Function{
			Declaration: &data.Declaration{
				Name: "Writefile",
				Parameters: []data.Parameter{
					{
						Name: "filename",
						Type: data.StringType,
					},
					{
						Name: "mode",
						Type: data.IntType,
					},
					{
						Name: "data",
						Type: data.ArrayType(data.ByteType),
					},
				},
				Returns: []*data.Type{data.ErrorType},
			},
			Value: writeFile,
		},
	})

	pkg, _ := bytecode.GetPackage(newpkg.Name)
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name, newpkg)
}

// Alternate initializer for the package that only creates the required Exit function.
func MinimalInitialize(s *symbols.SymbolTable) {
	newpkg := data.NewPackageFromMap("os", map[string]interface{}{
		"Exit": data.Function{
			Declaration: &data.Declaration{
				Name: "Exit",
				Parameters: []data.Parameter{
					{
						Name: "code",
						Type: data.IntType,
					},
				},
				ArgCount: data.Range{0, 1},
			},
			Value: exit,
		},
	})

	pkg, _ := bytecode.GetPackage(newpkg.Name)
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name, newpkg)
}
