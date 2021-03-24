package functions

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// ReadFile reads a file contents into a string value.
func ReadFile(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	name := util.GetString(args[0])
	if name == "." {
		return ui.Prompt(""), nil
	}

	content, err := ioutil.ReadFile(name)
	if !errors.Nil(err) {
		return nil, errors.New(err)
	}

	// Convert []byte to string
	return string(content), nil
}

// WriteFile writes a string to a file.
func WriteFile(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	fileName := util.GetString(args[0])
	text := util.GetString(args[1])
	err := ioutil.WriteFile(fileName, []byte(text), 0777)

	return len(text), errors.New(err)
}

// DeleteFile delete a file.
func DeleteFile(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	fileName := util.GetString(args[0])
	err := os.Remove(fileName)

	return errors.Nil(err), errors.New(err)
}

// Expand expands a list of file or path names into a list of files.
func Expand(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	path := util.GetString(args[0])
	ext := ""

	if len(args) > 1 {
		ext = util.GetString(args[1])
	}

	list, err := ExpandPath(path, ext)

	// Rewrap as an Ego array
	result := datatypes.NewArray(datatypes.StringTypeDef, 0)

	for _, item := range list {
		result.Append(item)
	}

	return result, err
}

// ExpandPath is used to expand a path into a list of fie names. This is
// also used elsewhere to product path lists, so it must be an exported
// symbol.
func ExpandPath(path, ext string) ([]string, *errors.EgoError) {
	names := []string{}

	// Can we read this as a directory?
	fi, err := ioutil.ReadDir(path)
	if !errors.Nil(err) {
		fn := path

		_, err := ioutil.ReadFile(fn)
		if !errors.Nil(err) {
			fn = path + ext
			_, err = ioutil.ReadFile(fn)
		}

		if !errors.Nil(err) {
			return names, errors.New(err)
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
		if !errors.Nil(err) {
			return names, err
		}

		names = append(names, list...)
	}

	return names, nil
}

// ReadDir implements the io.readdir() function.
func ReadDir(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	path := util.GetString(args[0])
	result := datatypes.NewArray(datatypes.InterfaceTypeDef, 0)

	files, err := ioutil.ReadDir(path)
	if !errors.Nil(err) {
		return result, errors.New(err).In("ReadDir()")
	}

	for _, file := range files {
		entry := datatypes.NewMap(datatypes.StringTypeDef, datatypes.InterfaceTypeDef)

		_, _ = entry.Set("name", file.Name())
		_, _ = entry.Set("directory", file.IsDir())
		_, _ = entry.Set("mode", file.Mode().String())
		_, _ = entry.Set("size", int(file.Size()))
		_, _ = entry.Set("modified", file.ModTime().String())

		result.Append(entry)
	}

	return result, nil
}

// This is the generic close() which can be used to close a channel, and maybe
// later other items as well.
func CloseAny(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	switch arg := args[0].(type) {
	case *datatypes.Channel:
		return arg.Close(), nil

	default:
		return nil, errors.New(errors.InvalidTypeError).In("CloseAny()")
	}
}
