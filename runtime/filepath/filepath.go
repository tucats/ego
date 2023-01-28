package filepath

import (
	"path/filepath"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Given a list of path components, connect them together in the syntax
// supported by the host platform as a file system path. Resolve duplicate
// separators.
func join(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	parts := make([]string, len(args))
	for i, v := range args {
		parts[i] = data.String(v)
	}

	return filepath.Join(parts...), nil
}

func base(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	path := data.String(args[0])

	return filepath.Base(path), nil
}

func abs(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	path := data.String(args[0])
	absPath, err := filepath.Abs(path)

	if err != nil {
		err = errors.NewError(err)
	}

	return absPath, err
}

func ext(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	path := data.String(args[0])

	return filepath.Ext(path), nil
}

func dir(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	path := data.String(args[0])

	return filepath.Dir(path), nil
}

func clean(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	path := data.String(args[0])

	return filepath.Clean(path), nil
}
