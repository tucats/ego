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

// openFile opens a file.
func openFile(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var (
		f         *os.File
		mask      os.FileMode = 0644
		mode                  = os.O_RDONLY
		modeValue             = "input"
	)

	fileName, err := filepath.Abs(sandboxName(data.String(args.Get(0))))
	if err != nil {
		err = errors.New(err).In("ReadDir")

		return data.NewList(nil, err), err
	}

	if args.Len() > 1 {
		modeValue = strings.ToLower(data.String(args.Get(1)))

		// Is it a valid mode name?
		if !util.InList(modeValue, "input", "read", "output", "write", "create", "append") {
			return nil, errors.ErrInvalidFileMode.Context(modeValue)
		}
		// If we are opening for output mode, delete the file if it already
		// exists
		if util.InList(modeValue, "create", "write", "output") {
			_ = os.Remove(fileName)
			mode = os.O_CREATE | os.O_WRONLY
			modeValue = "output"
		} else if modeValue == "append" {
			// For append, adjust the mode bits
			mode = os.O_APPEND | os.O_WRONLY
		} else {
			modeValue = "input"
		}
	}

	if args.Len() > 2 {
		mask = os.FileMode(data.IntOrZero(args.Get(2)) & math.MaxInt8)
	}

	f, err = os.OpenFile(fileName, mode, mask)
	if err != nil {
		err = errors.New(err).In("ReadDir")

		return data.NewList(nil, err), errors.New(err)
	}

	fileObject := data.NewStruct(IoFileType)
	fileObject.SetReadonly(true)
	fileObject.SetAlways(fileFieldName, f)
	fileObject.SetAlways(validFieldName, true)
	fileObject.SetAlways(nameFieldName, fileName)
	fileObject.SetAlways(modeFieldName, modeValue)

	return data.NewList(fileObject, nil), nil
}
