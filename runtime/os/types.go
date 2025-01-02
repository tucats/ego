package os

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

var initLock sync.Mutex

func Initialize(s *symbols.SymbolTable) {
	initLock.Lock()
	defer initLock.Unlock()

	if _, found := s.Root().Get("os"); !found {
		fileTypeDef := data.TypeDefinition("File", data.StructureType()).
			SetNativeName("os.File").
			SetPackage("os").
			SetFormatFunc(formatFileType)

		fileTypeDef.DefineNativeFunction("Read", &data.Declaration{
			Name: "Read",
			Type: fileTypeDef,
			Parameters: []data.Parameter{
				{
					Name: "buff",
					Type: data.ArrayType(data.ByteType),
				},
			},
			Returns: []*data.Type{data.IntType, data.ErrorType},
		}, nil)

		fileTypeDef.DefineNativeFunction("Chdir", &data.Declaration{
			Name:    "Chdir",
			Type:    fileTypeDef,
			Returns: []*data.Type{data.ErrorType},
		}, nil)

		fileTypeDef.DefineNativeFunction("Chown", &data.Declaration{
			Name: "Chown",
			Type: fileTypeDef,
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
		}, nil)

		fileTypeDef.DefineNativeFunction("Name", &data.Declaration{
			Name:    "Name",
			Type:    fileTypeDef,
			Returns: []*data.Type{data.StringType},
		}, nil)

		fileTypeDef.DefineNativeFunction("Close", &data.Declaration{
			Name:    "Close",
			Type:    fileTypeDef,
			Returns: []*data.Type{data.ErrorType},
		}, nil)

		fileTypeDef.DefineNativeFunction("ReadAt", &data.Declaration{
			Name: "ReadAt",
			Type: fileTypeDef,
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
		}, nil)

		fileTypeDef.DefineFunction("Write", &data.Declaration{
			Name: "Write",
			Type: fileTypeDef,
			Parameters: []data.Parameter{
				{
					Name: "bytes",
					Type: data.ArrayType(data.ByteType),
				},
			},
			Returns: []*data.Type{data.IntType, data.ErrorType},
		}, nil)

		fileTypeDef.DefineNativeFunction("WriteAt", &data.Declaration{
			Name: "WriteAt",
			Type: fileTypeDef,
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
		}, nil)

		fileTypeDef.DefineFunction("WriteString", &data.Declaration{
			Name: "WriteString",
			Type: fileTypeDef,
			Parameters: []data.Parameter{
				{
					Name: "s",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.IntType, data.ErrorType},
		}, nil)

		newpkg := data.NewPackageFromMap("os", map[string]interface{}{
			"File": fileTypeDef,
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
				Value:    os.Chdir,
				IsNative: true,
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
							Name: "path",
							Type: data.StringType,
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
					Returns: []*data.Type{data.PointerType(fileTypeDef)},
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
					Returns: []*data.Type{data.PointerType(fileTypeDef)},
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
				Value:    os.Executable,
				IsNative: true,
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
					Returns: []*data.Type{data.PointerType(fileTypeDef)},
				},
				Value:    os.Open,
				IsNative: true,
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
							Name:      "filename",
							Sandboxed: true,
							Type:      data.StringType,
						},
					},
					Returns: []*data.Type{data.ErrorType},
				},
				Value:    os.Remove,
				IsNative: true,
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

func formatFileInfoType(v interface{}) string {
	fi, ok := v.(os.FileInfo)
	if !ok || fi == nil {
		return "os.FileInfo{nil}"
	}

	text := "os.FileInfo{Name:" + fi.Name()
	text += ", Modified: " + fi.ModTime().Format(time.RFC3339)
	text += ", Size:" + strconv.FormatInt(fi.Size(), 10)
	text += ", Mode: " + fi.Mode().String()

	return text + "}"
}
func formatFileType(v interface{}) string {
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
