package functions

import (
	"path/filepath"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Given a list of path components, connect them together in the syntax
// supported by the host platform as a file system path. Resolve duplicate
// separators.
func PathJoin(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	parts := make([]string, len(args))
	for i, v := range args {
		parts[i] = datatypes.String(v)
	}

	return filepath.Join(parts...), nil
}

func PathBase(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	path := datatypes.String(args[0])

	return filepath.Base(path), nil
}

func PathAbs(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	path := datatypes.String(args[0])
	absPath, err := filepath.Abs(path)

	if err != nil {
		err = errors.EgoError(err)
	}

	return absPath, err
}

func PathExt(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	path := datatypes.String(args[0])

	return filepath.Ext(path), nil
}

func PathDir(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	path := datatypes.String(args[0])

	return filepath.Dir(path), nil
}

func PathClean(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	path := datatypes.String(args[0])

	return filepath.Clean(path), nil
}
