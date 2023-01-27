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
func Join(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) == 0 {
		return nil, errors.ErrArgumentCount.In("Join()")
	}

	parts := make([]string, len(args))
	for i, v := range args {
		parts[i] = data.String(v)
	}

	return filepath.Join(parts...), nil
}

func Base(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount.In("Base()")
	}

	path := data.String(args[0])

	return filepath.Base(path), nil
}

func Abs(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount.In("Abs()")
	}

	path := data.String(args[0])
	absPath, err := filepath.Abs(path)

	if err != nil {
		err = errors.NewError(err)
	}

	return absPath, err
}

func Ext(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount.In("Ext()")
	}

	path := data.String(args[0])

	return filepath.Ext(path), nil
}

func Dir(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount.In("Dir()")
	}

	path := data.String(args[0])

	return filepath.Dir(path), nil
}

func Clean(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount.In("Clean()")
	}

	path := data.String(args[0])

	return filepath.Clean(path), nil
}
