package functions

import (
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// ReadFile reads a file contents into a string value.
func ReadFile(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount
	}

	name := data.String(args[0])
	if name == "." {
		return ui.Prompt(""), nil
	}

	name = sandboxName(name)

	content, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, errors.NewError(err)
	}

	return data.NewArray(data.ByteType, 0).Append(content), nil
}

// WriteFile writes a string to a file.
func WriteFile(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 2 {
		return nil, errors.ErrArgumentCount
	}

	fileName := sandboxName(data.String(args[0]))

	if a, ok := args[1].(*data.Array); ok {
		if a.ValueType().Kind() == data.ByteKind {
			err := ioutil.WriteFile(fileName, a.GetBytes(), 0777)
			if err != nil {
				err = errors.NewError(err)
			}

			return a.Len(), err
		}
	}

	text := data.String(args[1])

	err := ioutil.WriteFile(fileName, []byte(text), 0777)
	if err != nil {
		err = errors.NewError(err)
	}

	return len(text), err
}

// DeleteFile deletes a file.
func DeleteFile(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount
	}

	fileName := data.String(args[0])
	fileName = sandboxName(fileName)

	err := os.Remove(fileName)
	if err != nil {
		err = errors.NewError(err)
	}

	return err == nil, err
}

// Expand expands a list of file or path names into a list of files.
func Expand(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 2 {
		return nil, errors.ErrArgumentCount
	}

	path := data.String(args[0])
	ext := ""

	if len(args) > 1 {
		ext = data.String(args[1])
	}

	path = sandboxName(path)
	list, err := ExpandPath(path, ext)

	// Rewrap as an Ego array
	result := data.NewArray(data.StringType, 0)

	for _, item := range list {
		result.Append(item)
	}

	return result, err
}

// ExpandPath is used to expand a path into a list of file names. This is
// also used elsewhere to product path lists, so it must be an exported
// symbol.
func ExpandPath(path, ext string) ([]string, error) {
	names := []string{}

	path = sandboxName(path)

	// Can we read this as a directory?
	fi, err := ioutil.ReadDir(path)
	if err != nil {
		fn := path

		_, err := ioutil.ReadFile(fn)
		if err != nil {
			fn = path + ext
			_, err = ioutil.ReadFile(fn)
		}

		if err != nil {
			return names, errors.NewError(err)
		}

		// If we have a default suffix, make sure the pattern matches
		if ext != "" && !strings.HasSuffix(fn, ext) {
			return names, nil
		}

		names = append(names, fn)

		return names, nil
	}

	// Read as a directory
	for _, f := range fi {
		fn := filepath.Join(path, f.Name())

		list, err := ExpandPath(fn, ext)
		if err != nil {
			return names, err
		}

		names = append(names, list...)
	}

	return names, nil
}

// ReadDir implements the io.readdir() function.
func ReadDir(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	path := data.String(args[0])
	result := data.NewArray(data.InterfaceType, 0)

	path = sandboxName(path)

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return result, errors.NewError(err).In("ReadDir()")
	}

	for _, file := range files {
		entry := data.NewMap(data.StringType, data.InterfaceType)

		_, _ = entry.Set("name", file.Name())
		_, _ = entry.Set("directory", file.IsDir())
		_, _ = entry.Set("mode", file.Mode().String())
		_, _ = entry.Set("size", int(file.Size()))
		_, _ = entry.Set("modified", file.ModTime().String())

		result.Append(entry)
	}

	return result, nil
}

// Chdir implements the os.Chdir() function.
func Chdir(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	path := data.String(args[0])

	if err := os.Chdir(path); err != nil {
		return nil, errors.NewError(err).In("Chdir")
	}

	return nil, nil
}

// Chmod implements the os.Chmod() function.
func Chmod(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	path := data.String(args[0])
	mode := data.Int32(args[1])

	if err := os.Chmod(path, fs.FileMode(mode)); err != nil {
		return nil, errors.NewError(err).In("Chmod")
	}

	return nil, nil
}

// Chown implements the os.Chown() function.
func Chown(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	path := data.String(args[0])
	uid := data.Int(args[1])
	gid := data.Int(args[1])

	if err := os.Chown(path, uid, gid); err != nil {
		return nil, errors.NewError(err).In("Chown")
	}

	return nil, nil
}

// Clearenv implements the os.Clearenv() function.
func Clearenv(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	os.Clearenv()

	return nil, nil
}

// Environ implements the os.Environ() function.
func Environ(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	env := os.Environ()

	result := data.NewArray(data.StringType, len(env))

	for index, value := range env {
		if err := result.Set(index, value); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// Executable implements the os.Executable() function.
func Executable(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	path, err := os.Executable()

	if err != nil {
		err = errors.NewError(err)
	}

	return path, err
}

// This is the generic close() which can be used to close a channel or a file,
// and maybe later other items as well.
func CloseAny(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	switch arg := args[0].(type) {
	case *data.Channel:
		return arg.Close(), nil

	case *data.Struct:
		switch arg.TypeString() {
		case "io.File":
			s.SetAlways("__this", arg)

			return Close(s, []interface{}{})

		default:
			return nil, errors.ErrInvalidType.In("close()").Context(arg.TypeString())
		}

	default:
		return nil, errors.ErrInvalidType.In("CloseAny()")
	}
}

func sandboxName(path string) string {
	if sandboxPrefix := settings.Get(defs.SandboxPathSetting); sandboxPrefix != "" {
		if strings.HasPrefix(path, sandboxPrefix) {
			return path
		}

		return filepath.Join(sandboxPrefix, path)
	}

	return path
}
