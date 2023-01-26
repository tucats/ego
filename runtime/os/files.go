package os

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

// Readfile implements os.REadFile() which reads a file contents into a
// byte array value.
func Readfile(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount
	}

	name := data.String(args[0])
	if name == "." {
		return ui.Prompt(""), nil
	}

	name = sandboxName(name)

	content, err := os.ReadFile(name)
	if err != nil {
		return nil, errors.NewError(err)
	}

	return data.NewArray(data.ByteType, 0).Append(content), nil
}

// Writefile implements os.Writefile() writes a byte array (or string) to a file.
func Writefile(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
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

	err := os.WriteFile(fileName, []byte(text), 0777)
	if err != nil {
		err = errors.NewError(err)
	}

	return len(text), err
}

// DeleteFile deletes a file.
func Remove(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
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

func sandboxName(path string) string {
	if sandboxPrefix := settings.Get(defs.SandboxPathSetting); sandboxPrefix != "" {
		if strings.HasPrefix(path, sandboxPrefix) {
			return path
		}

		return filepath.Join(sandboxPrefix, path)
	}

	return path
}
