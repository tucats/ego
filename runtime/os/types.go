package os

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func Initialize(s *symbols.SymbolTable) {
	newpkg := data.NewPackageFromMap("os", map[string]interface{}{
		"Args": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name:        "Args",
				ReturnTypes: []*data.Type{data.ArrayType(data.StringType)},
			},
			Value: Args,
		},
		"Chdir": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Chdir",
				Parameters: []data.FunctionParameter{
					{
						Name:     "path",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{data.ErrorType},
			},
			Value: Chdir,
		},
		"Chmod": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Chmod",
				Parameters: []data.FunctionParameter{
					{
						Name:     "file",
						ParmType: data.StringType,
					},
					{
						Name:     "mode",
						ParmType: data.IntType,
					},
				},
				ReturnTypes: []*data.Type{data.ErrorType},
			},
			Value: Chmod,
		},
		"Chown": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Chown",
				Parameters: []data.FunctionParameter{
					{
						Name:     "file",
						ParmType: data.StringType,
					},
					{
						Name:     "owner",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{data.ErrorType},
			},
			Value: Chown,
		},
		"Clearenv": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Clearenv",
			},
			Value: Clearenv,
		},
		"Environ": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name:        "Environ",
				ReturnTypes: []*data.Type{data.ArrayType(data.StringType)},
			},
			Value: Environ,
		},
		"Executable": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name:        "Executable",
				ReturnTypes: []*data.Type{data.StringType},
			},
			Value: Executable,
		},
		"Exit": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Exit",
				Parameters: []data.FunctionParameter{
					{
						Name:     "code",
						ParmType: data.IntType,
					},
				},
			},
			Value: Exit,
		},
		"Getenv": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Getenv",
				Parameters: []data.FunctionParameter{
					{
						Name:     "name",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{data.StringType},
			},
			Value: Getenv,
		},
		"Hostname": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name:        "Hostname",
				ReturnTypes: []*data.Type{data.StringType},
			},
			Value: Hostname,
		},
		"ReadFile": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "ReadFile",
				Parameters: []data.FunctionParameter{
					{
						Name:     "filename",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{data.ArrayType(data.ByteType), data.ErrorType},
			},
			Value: Readfile,
		},
		"Remove": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Remove",
				Parameters: []data.FunctionParameter{
					{
						Name:     "filename",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{data.ErrorType},
			},
			Value: Remove,
		},
		"Writefile": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Writefile",
				Parameters: []data.FunctionParameter{
					{
						Name:     "filename",
						ParmType: data.StringType,
					},
					{
						Name:     "mode",
						ParmType: data.IntType,
					},
					{
						Name:     "data",
						ParmType: data.ArrayType(data.ByteType),
					},
				},
				ReturnTypes: []*data.Type{data.ErrorType},
			},
			Value: Writefile,
		},
	})

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
