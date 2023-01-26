package functions

import (
	"path/filepath"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// This is the generic close() which can be used to close a channel or a file,
// and maybe later other items as well.
func CloseAny(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	switch arg := args[0].(type) {
	case *data.Channel:
		return arg.Close(), nil

	case *data.Struct:
		switch arg.TypeString() {
		case "io.File":
			s.SetAlways(defs.ThisVariable, arg)

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
