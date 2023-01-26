package io

import (
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// Open opens a file.
func Open(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var mask os.FileMode = 0644

	var f *os.File

	mode := os.O_RDONLY

	fname, err := filepath.Abs(sandboxName(data.String(args[0])))
	if err != nil {
		return data.List(nil, err), errors.NewError(err)
	}

	modeValue := "input"

	if len(args) > 1 {
		modeValue = strings.ToLower(data.String(args[1]))

		// Is it a valid mode name?
		if !util.InList(modeValue, "input", "read", "output", "write", "create", "append") {
			return nil, errors.ErrInvalidFileMode.Context(modeValue)
		}
		// If we are opening for output mode, delete the file if it already
		// exists
		if util.InList(modeValue, "create", "write", "output") {
			_ = os.Remove(fname)
			mode = os.O_CREATE | os.O_WRONLY
			modeValue = "output"
		} else if modeValue == "append" {
			// For append, adjust the mode bits
			mode = os.O_APPEND | os.O_WRONLY
		} else {
			modeValue = "input"
		}
	}

	if len(args) > 2 {
		mask = os.FileMode(data.Int(args[2]) & math.MaxInt8)
	}

	f, err = os.OpenFile(fname, mode, mask)
	if err != nil {
		return data.List(nil, err), errors.NewError(err)
	}

	fobj := data.NewStruct(fileType)
	fobj.SetReadonly(true)
	fobj.SetAlways(fileFieldName, f)
	fobj.SetAlways(validFieldName, true)
	fobj.SetAlways(nameFieldName, fname)
	fobj.SetAlways(modeFieldName, modeValue)

	return data.List(fobj, nil), nil
}
