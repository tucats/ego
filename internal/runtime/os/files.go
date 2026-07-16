package os

import (
	"io/fs"
	"math"
	"os"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/runtime/time"
	"github.com/tucats/ego/internal/util"
)

// SandBoxedIO returns the state of the sandbox flag for the current call, as
// set by the running Context (see callRuntimeFunction.go). Mirrors
// runtime/io's identically-named helper.
func SandBoxedIO(s *symbols.SymbolTable) bool {
	if v, ok := s.Get(defs.SandboxedIOSymbolName); ok {
		return data.BoolOrFalse(v)
	}

	return false
}

// readFile implements os.ReadFile() which reads a file contents into a
// byte array value.
func readFile(s *symbols.SymbolTable, args data.List) (any, error) {
	name := data.String(args.Get(0))
	if name == "." {
		return data.NewList(ui.Prompt(""), nil), nil
	}

	name = sandboxName(SandBoxedIO(s), name)

	content, err := os.ReadFile(name)
	if err != nil {
		err = errors.New(err).In("ReadFile")

		return data.NewList(nil, err), err
	}

	return data.NewList(data.NewArray(data.ByteType, 0).Append(content), nil), nil
}

// stat implements os.Stat(), which returns file info about a path without
// opening it. The path is resolved the same as any other sandboxed path
// argument (readFile, writeFile, changeMode).
func stat(s *symbols.SymbolTable, args data.List) (any, error) {
	name := sandboxName(SandBoxedIO(s), data.String(args.Get(0)))

	info, err := os.Stat(name)
	if err != nil {
		err = errors.New(err).In("Stat")

		return data.NewList(nil, err), err
	}

	fileInfo := data.NewStruct(OsFileInfoType)
	_ = fileInfo.Set("Name", info.Name())
	_ = fileInfo.Set("Size", info.Size())
	_ = fileInfo.Set("Mode", int(info.Mode()))
	_ = fileInfo.Set("ModTime", data.NewStruct(time.TimeType).SetNative(info.ModTime()))
	_ = fileInfo.Set("IsDir", info.IsDir())

	return data.NewList(fileInfo, nil), nil
}

// writeFile implements os.WriteFile() writes a byte array (or string) to a file.
// Accepting a string as the data parameter is an Ego extension.
func writeFile(s *symbols.SymbolTable, args data.List) (any, error) {
	fileName := sandboxName(SandBoxedIO(s), data.String(args.Get(0)))

	// The file mode must be a valid uint32 value.
	modeArg, err := args.GetInt(2)
	if err != nil {
		return errors.ErrInvalidFunctionArgument.In("WriteFile").Context(modeArg), nil
	}

	if modeArg < 0 || modeArg > math.MaxInt32 {
		return errors.ErrInvalidFunctionArgument.In("WriteFile").Context(modeArg), nil
	}

	mode := fs.FileMode(uint32(modeArg))

	if a, ok := args.Get(1).(*data.Array); ok {
		if a.Type().Kind() == data.ByteKind {
			if err := os.WriteFile(fileName, a.GetBytes(), mode); err != nil {
				return errors.New(err).In("WriteFile"), nil
			}

			return nil, nil
		}
	}

	text := data.String(args.Get(1))

	if err := os.WriteFile(fileName, []byte(text), mode); err != nil {
		return errors.New(err).In("WriteFile"), nil
	}

	return nil, nil
}

// changeMode implements the os.changeMode() function.
func changeMode(s *symbols.SymbolTable, args data.List) (any, error) {
	path := sandboxName(SandBoxedIO(s), data.String(args.Get(0)))

	// The file mode must be a valid uint32 value.
	modeArg, err := args.GetInt(1)
	if err != nil {
		return errors.ErrInvalidFunctionArgument.In("Chmod").Context(modeArg), nil
	}

	if modeArg < 0 || modeArg > math.MaxInt32 {
		return errors.ErrInvalidFunctionArgument.In("Chmod").Context(modeArg), nil
	}

	mode := fs.FileMode(uint32(modeArg))
	if err := os.Chmod(path, mode); err != nil {
		return errors.New(err).In("Chmod"), nil
	}

	return nil, nil
}

// mkdir implements os.Mkdir(), creating a single directory. The path is
// resolved the same as any other sandboxed path argument (readFile, writeFile,
// changeMode), and the mode is validated exactly like changeMode's.
func mkdir(s *symbols.SymbolTable, args data.List) (any, error) {
	path := sandboxName(SandBoxedIO(s), data.String(args.Get(0)))

	mode, err := dirMode(args, 1, "Mkdir")
	if err != nil {
		return err, nil
	}

	if err := os.Mkdir(path, mode); err != nil {
		return errors.New(err).In("Mkdir"), nil
	}

	return nil, nil
}

// mkdirAll implements os.MkdirAll(), creating a directory along with any parent
// directories that do not yet exist. Like Go's os.MkdirAll, it is not an error
// if the path already exists as a directory.
func mkdirAll(s *symbols.SymbolTable, args data.List) (any, error) {
	path := sandboxName(SandBoxedIO(s), data.String(args.Get(0)))

	mode, err := dirMode(args, 1, "MkdirAll")
	if err != nil {
		return err, nil
	}

	if err := os.MkdirAll(path, mode); err != nil {
		return errors.New(err).In("MkdirAll"), nil
	}

	return nil, nil
}

// dirMode validates and converts a directory-permission argument, mirroring the
// uint32-range check used by writeFile and changeMode. The returned error is an
// Ego error tagged with the calling function's name, ready to be returned as
// the wrapper's error-typed result value.
func dirMode(args data.List, index int, fnName string) (fs.FileMode, error) {
	modeArg, err := args.GetInt(index)
	if err != nil {
		return 0, errors.ErrInvalidFunctionArgument.In(fnName).Context(modeArg)
	}

	if modeArg < 0 || modeArg > math.MaxInt32 {
		return 0, errors.ErrInvalidFunctionArgument.In(fnName).Context(modeArg)
	}

	return fs.FileMode(uint32(modeArg)), nil
}

func sandboxName(flag bool, path string) string {
	sandboxPrefix := settings.Get(defs.SandboxPathSetting)
	if !flag || sandboxPrefix == "" {
		return path
	}

	return util.SandboxJoin(sandboxPrefix, path)
}
