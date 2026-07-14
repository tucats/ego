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

func sandboxName(flag bool, path string) string {
	sandboxPrefix := settings.Get(defs.SandboxPathSetting)
	if !flag || sandboxPrefix == "" {
		return path
	}

	return util.SandboxJoin(sandboxPrefix, path)
}
