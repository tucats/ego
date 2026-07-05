package os

import (
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// readFile implements os.ReadFile() which reads a file contents into a
// byte array value.
func readFile(s *symbols.SymbolTable, args data.List) (any, error) {
	name := data.String(args.Get(0))
	if name == "." {
		return data.NewList(ui.Prompt(""), nil), nil
	}

	name = sandboxName(name)

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
	fileName := sandboxName(data.String(args.Get(0)))

	// The file mode must be a valid uint32 value.
	modeArg, err := args.GetInt(2)
	if err != nil {
		return nil, errors.ErrInvalidFunctionArgument.In("os.WriteFile").Context(modeArg)
	}

	if modeArg < 0 || modeArg > math.MaxInt32 {
		return nil, errors.ErrInvalidFunctionArgument.In("WriteFile").Context(modeArg)
	}

	mode := fs.FileMode(uint32(modeArg))

	if a, ok := args.Get(1).(*data.Array); ok {
		if a.Type().Kind() == data.ByteKind {
			if err := os.WriteFile(fileName, a.GetBytes(), mode); err != nil {
				err = errors.New(err).In("WriteFile")

				return err, err
			}

			return nil, nil
		}
	}

	text := data.String(args.Get(2))

	err = os.WriteFile(fileName, []byte(text), mode)
	if err != nil {
		err = errors.New(err).In("WriteFile")
	}

	return err, err
}

// changeMode implements the os.changeMode() function.
func changeMode(s *symbols.SymbolTable, args data.List) (any, error) {
	path := data.String(args.Get(0))

	// The file mode must be a valid uint32 value.
	modeArg, err := args.GetInt(1)
	if err != nil {
		return nil, errors.ErrInvalidFunctionArgument.In("os.changeMode").Context(modeArg)
	}

	if modeArg < 0 || modeArg > math.MaxInt32 {
		return nil, errors.ErrInvalidFunctionArgument.In("WriteFile").Context(modeArg)
	}

	mode := fs.FileMode(uint32(modeArg))
	if err := os.Chmod(path, fs.FileMode(mode)); err != nil {
		return nil, errors.New(err).In("Chmod")
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
