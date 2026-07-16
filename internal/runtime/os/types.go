package os

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/tucats/ego/internal/language/data"
	egotime "github.com/tucats/ego/internal/runtime/time"
)

// OsFileInfoType describes the result of os.Stat(): a plain field-based
// struct (matching the io.Entry convention in internal/runtime/io/types.go)
// rather than Go's method-based os.FileInfo interface. Mode is the raw
// permission/type bits as an int, the same representation os.Chmod()'s mode
// parameter already uses, so os.Chmod(path, info.Mode) round-trips directly.
var OsFileInfoType = data.TypeDefinition("FileInfo",
	data.StructureType().
		DefineField("Name", data.StringType).
		DefineField("Size", data.Int64Type).
		DefineField("Mode", data.IntType).
		DefineField("ModTime", egotime.TimeType).
		DefineField("IsDir", data.BoolType)).
	SetPackage("os")

var OsFileType = data.TypeDefinition("File", data.StructureType()).
	SetNativeName("os.File").
	SetPackage("os").
	SetFormatFunc(formatFileType).
	DefineNativeFunction("Read", &data.Declaration{
		Name: "Read",
		Type: data.OwnType,
		Parameters: []data.Parameter{
			{
				Name: "buff",
				Type: data.ArrayType(data.ByteType),
			},
		},
		Returns: []*data.Type{data.IntType, data.ErrorType},
	}, nil).
	DefineNativeFunction("Chdir", &data.Declaration{
		Name:    "Chdir",
		Type:    data.OwnType,
		Returns: []*data.Type{data.ErrorType},
	}, nil).
	DefineNativeFunction("Chown", &data.Declaration{
		Name: "Chown",
		Type: data.OwnType,
		Parameters: []data.Parameter{
			{
				Name: "uid",
				Type: data.IntType,
			},
			{
				Name: "gid",
				Type: data.IntType,
			},
		},
		Returns: []*data.Type{data.ErrorType},
	}, nil).
	DefineNativeFunction("Name", &data.Declaration{
		Name:    "Name",
		Type:    data.OwnType,
		Returns: []*data.Type{data.StringType},
	}, nil).
	DefineNativeFunction("Close", &data.Declaration{
		Name:    "Close",
		Type:    data.OwnType,
		Returns: []*data.Type{data.ErrorType},
	}, nil).
	DefineNativeFunction("ReadAt", &data.Declaration{
		Name: "ReadAt",
		Type: data.OwnType,
		Parameters: []data.Parameter{
			{
				Name: "buff",
				Type: data.ArrayType(data.ByteType),
			},
			{
				Name: "offset",
				Type: data.Int64Type,
			},
		},
		Returns: []*data.Type{data.IntType, data.ErrorType},
	}, nil).
	DefineNativeFunction("Write", &data.Declaration{
		Name: "Write",
		Type: data.OwnType,
		Parameters: []data.Parameter{
			{
				Name: "bytes",
				Type: data.ArrayType(data.ByteType),
			},
		},
		Returns: []*data.Type{data.IntType, data.ErrorType},
	}, nil).
	DefineNativeFunction("WriteAt", &data.Declaration{
		Name: "WriteAt",
		Type: data.OwnType,
		Parameters: []data.Parameter{
			{
				Name: "bytes",
				Type: data.ArrayType(data.ByteType),
			},
			{
				Name: "offset",
				Type: data.Int64Type,
			},
		},
		Returns: []*data.Type{data.IntType, data.ErrorType},
	}, nil).
	DefineNativeFunction("WriteString", &data.Declaration{
		Name: "WriteString",
		Type: data.OwnType,
		Parameters: []data.Parameter{
			{
				Name: "s",
				Type: data.StringType,
			},
		},
		Returns: []*data.Type{data.IntType, data.ErrorType},
	}, nil).FixSelfReferences()

var OsPackage = data.NewPackageFromMap("os", map[string]any{
	"File":     OsFileType,
	"FileInfo": OsFileInfoType,
	"Args": data.Function{
		Declaration: &data.Declaration{
			Name:    "Args",
			Returns: []*data.Type{data.ArrayType(data.StringType)},
		},
		Sandboxed: true,
		Value:     args,
	},
	"Chdir": data.Function{
		Declaration: &data.Declaration{
			Name: "Chdir",
			Parameters: []data.Parameter{
				{
					Name:      "path",
					Type:      data.StringType,
					Sandboxed: true,
				},
			},
			Returns: []*data.Type{data.ErrorType},
		},
		Value:    os.Chdir,
		IsNative: true,
	},
	"Chmod": data.Function{
		Declaration: &data.Declaration{
			Name: "Chmod",
			Parameters: []data.Parameter{
				{
					Name:      "file",
					Type:      data.StringType,
					Sandboxed: true,
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
					Name:      "path",
					Type:      data.StringType,
					Sandboxed: true,
				},
				{
					Name: "uid",
					Type: data.IntType,
				},
				{
					Name: "gid",
					Type: data.IntType,
				},
			},
			Returns: []*data.Type{data.ErrorType},
		},
		Value:    os.Chown,
		IsNative: true,
	},
	"Clearenv": data.Function{
		Declaration: &data.Declaration{
			Name: "Clearenv",
		},
		Value:    os.Clearenv,
		IsNative: true,
	},
	"Create": data.Function{
		Declaration: &data.Declaration{
			Name: "Create",
			Parameters: []data.Parameter{
				{
					Name:      "name",
					Sandboxed: true,
					Type:      data.StringType,
				},
			},
			Returns: []*data.Type{data.PointerType(OsFileType)},
		},
		Value:    os.Create,
		IsNative: true,
	},
	"CreateTemp": data.Function{
		Declaration: &data.Declaration{
			Name: "CreateTemp",
			Parameters: []data.Parameter{
				{
					Name:      "dir",
					Sandboxed: true,
					Type:      data.StringType,
				},
				{
					Name: "pattern",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.PointerType(OsFileType)},
		},
		Value:    os.CreateTemp,
		IsNative: true,
	},
	"Environ": data.Function{
		Declaration: &data.Declaration{
			Name:    "Environ",
			Returns: []*data.Type{data.ArrayType(data.StringType)},
		},
		Value:    os.Environ,
		IsNative: true,
	},
	"Executable": data.Function{
		Declaration: &data.Declaration{
			Name:    "Executable",
			Returns: []*data.Type{data.StringType},
		},
		Value:     os.Executable,
		Sandboxed: true,
		IsNative:  true,
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
	"Expand": data.Function{
		Declaration: &data.Declaration{
			Name:  "Expand",
			Scope: true,
			Parameters: []data.Parameter{
				{
					Name: "s",
					Type: data.StringType,
				},
				{
					Name: "lookup",
					Type: data.FunctionType(&data.Function{
						Declaration: &data.Declaration{
							Name: "",
							Parameters: []data.Parameter{
								{
									Name: "key",
									Type: data.StringType,
								},
							},
							Returns: []*data.Type{data.StringType},
						},
					}),
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value: expand,
	},
	"ExpandEnv": data.Function{
		Declaration: &data.Declaration{
			Name: "ExpandEnv",
			Parameters: []data.Parameter{
				{
					Name: "s",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value:    os.ExpandEnv,
		IsNative: true,
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
		Value:    os.Getenv,
		IsNative: true,
	},
	"Hostname": data.Function{
		Declaration: &data.Declaration{
			Name:    "Hostname",
			Returns: []*data.Type{data.StringType, data.ErrorType},
		},
		Value:    os.Hostname,
		IsNative: true,
	},
	"LookupEnv": data.Function{
		Declaration: &data.Declaration{
			Name: "LookupEnv",
			Parameters: []data.Parameter{
				{
					Name: "name",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.StringType, data.BoolType},
		},
		Value:    os.LookupEnv,
		IsNative: true,
	},
	"Mkdir": data.Function{
		Declaration: &data.Declaration{
			Name: "Mkdir",
			Parameters: []data.Parameter{
				{
					Name:      "path",
					Type:      data.StringType,
					Sandboxed: true,
				},
				{
					Name: "perm",
					Type: data.IntType,
				},
			},
			Returns: []*data.Type{data.ErrorType},
		},
		Value: mkdir,
	},
	"MkdirAll": data.Function{
		Declaration: &data.Declaration{
			Name: "MkdirAll",
			Parameters: []data.Parameter{
				{
					Name:      "path",
					Type:      data.StringType,
					Sandboxed: true,
				},
				{
					Name: "perm",
					Type: data.IntType,
				},
			},
			Returns: []*data.Type{data.ErrorType},
		},
		Value: mkdirAll,
	},
	"Open": data.Function{
		Declaration: &data.Declaration{
			Name: "Open",
			Parameters: []data.Parameter{
				{
					Name:      "name",
					Sandboxed: true,
					Type:      data.StringType,
				},
			},
			Returns: []*data.Type{data.PointerType(OsFileType)},
		},
		Value:    os.Open,
		IsNative: true,
	},
	"ReadFile": data.Function{
		Declaration: &data.Declaration{
			Name: "ReadFile",
			Parameters: []data.Parameter{
				{
					Name:      "filename",
					Type:      data.StringType,
					Sandboxed: true,
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
					Name:      "filename",
					Sandboxed: true,
					Type:      data.StringType,
				},
			},
			Returns: []*data.Type{data.ErrorType},
		},
		Value:     os.Remove,
		Sandboxed: true,
		IsNative:  true,
	},
	"RemoveAll": data.Function{
		Declaration: &data.Declaration{
			Name: "RemoveAll",
			Parameters: []data.Parameter{
				{
					Name:      "path",
					Sandboxed: true,
					Type:      data.StringType,
				},
			},
			Returns: []*data.Type{data.ErrorType},
		},
		Value:     os.RemoveAll,
		Sandboxed: true,
		IsNative:  true,
	},
	"Setenv": data.Function{
		Declaration: &data.Declaration{
			Name: "Setenv",
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
			Returns: []*data.Type{data.ErrorType},
		},
		Value:    os.Setenv,
		IsNative: true,
	},
	"Stat": data.Function{
		Declaration: &data.Declaration{
			Name: "Stat",
			Parameters: []data.Parameter{
				{
					Name:      "name",
					Type:      data.StringType,
					Sandboxed: true,
				},
			},
			Returns: []*data.Type{OsFileInfoType, data.ErrorType},
		},
		Value: stat,
	},
	"TempDir": data.Function{
		Declaration: &data.Declaration{
			Name:    "TempDir",
			Returns: []*data.Type{data.StringType},
		},
		Value:    os.TempDir,
		IsNative: true,
	},
	"WriteFile": data.Function{
		Declaration: &data.Declaration{
			Name: "WriteFile",
			Parameters: []data.Parameter{
				{
					Name:      "filename",
					Type:      data.StringType,
					Sandboxed: true,
				},
				{
					// Declared as InterfaceType (not ArrayType(ByteType)) even
					// though a []byte is the common case, so that the string-data
					// extension (see writeFile) is accepted in strict type mode
					// too, not just dynamic mode.
					Name: "data",
					Type: data.InterfaceType,
				},
				{
					Name: "mode",
					Type: data.IntType,
				},
			},
			Returns: []*data.Type{data.ErrorType},
		},
		Value: writeFile,
	},
})

var OsMinimumPackage = data.NewPackageFromMap("os", map[string]any{
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

func formatFileType(v any) string {
	f := v.(*os.File)
	if f == nil {
		return "*os.File{nil}"
	}

	name := f.Name()
	if fullname, err := filepath.Abs(f.Name()); err == nil {
		name = fullname
	}

	text := "*os.File{Name:" + name

	info, _ := f.Stat()
	if info != nil {
		if info.IsDir() {
			text += ", IsDir:true"
		}

		text += ", Modified: " + info.ModTime().Format(time.RFC3339)
		text += ", Size:" + strconv.FormatInt(info.Size(), 10)
		text += ", Mode: " + info.Mode().String()
	}

	return text + "}"
}
