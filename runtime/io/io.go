package io

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// expand expands a list of file or path names into a list of files.
func expand(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	path := data.String(args.Get(0))
	ext := ""

	if args.Len() > 1 {
		ext = data.String(args.Get(1))
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
	fi, err := os.ReadDir(path)
	if err != nil {
		fn := path

		_, err := os.ReadFile(fn)
		if err != nil {
			fn = path + ext
			_, err = os.ReadFile(fn)
		}

		if err != nil {
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
		if err != nil {
			return names, err
		}

		names = append(names, list...)
	}

	return names, nil
}

// readDirectory implements the io.readdir() function.
func readDirectory(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	path := data.String(args.Get(0))
	result := data.NewArray(entryType, 0)

	path = sandboxName(path)

	files, err := os.ReadDir(path)
	if err != nil {
		err = errors.New(err).In("ReadDir")

		return data.NewList(result, err), err
	}

	for _, file := range files {
		entry := data.NewStruct(entryType)
		i, _ := file.Info()

		_ = entry.Set("Name", file.Name())
		_ = entry.Set("IsDirectory", file.IsDir())
		_ = entry.Set("Mode", i.Mode().String())
		_ = entry.Set("Size", int(i.Size()))
		_ = entry.Set("Modified", i.ModTime().String())

		result.Append(entry)
	}

	return data.NewList(result, nil), nil
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
