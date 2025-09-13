package os

import (
	"io/fs"
	"math"
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

// readFile implements os.ReadFile() which reads a file contents into a
// byte array value.
func readFile(s *symbols.SymbolTable, args data.List) (any, error) {
	name := data.String(args.Get(0))
	if name == "." {
		return ui.Prompt(""), nil
	}

	name = sandboxName(name)

	content, err := os.ReadFile(name)
	if err != nil {
		err = errors.New(err).In("ReadFile")

		return nil, err
	}

	return data.NewArray(data.ByteType, 0).Append(content), nil
}

// writeFile implements os.Writefile() writes a byte array (or string) to a file.
func writeFile(s *symbols.SymbolTable, args data.List) (any, error) {
	fileName := sandboxName(data.String(args.Get(0)))

	// The file mode must be a valid uint32 value.
	modeArg, err := args.GetInt(1)
	if err != nil {
		return nil, errors.ErrInvalidFunctionArgument.In("os.WriteFile").Context(modeArg)
	}

	if modeArg < 0 || modeArg > math.MaxInt32 {
		return nil, errors.ErrInvalidFunctionArgument.In("WriteFile").Context(modeArg)
	}

	mode := fs.FileMode(uint32(modeArg))

	if a, ok := args.Get(2).(*data.Array); ok {
		if a.Type().Kind() == data.ByteKind {
			if err := os.WriteFile(fileName, a.GetBytes(), mode); err != nil {
				err = errors.New(err).In("Writefile")

				return err, err
			}

			return nil, nil
		}
	}

	text := data.String(args.Get(2))

	err = os.WriteFile(fileName, []byte(text), mode)
	if err != nil {
		err = errors.New(err).In("Writefile")
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
