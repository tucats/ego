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
func join(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	parts := make([]string, args.Len())
	for i, v := range args.Elements() {
		parts[i] = data.String(v)
	}

	return filepath.Join(parts...), nil
}

func base(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	path := data.String(args.Get(0))

	return filepath.Base(path), nil
}

func abs(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	path := data.String(args.Get(0))
	absPath, err := filepath.Abs(path)

	if err != nil {
		err = errors.New(err)
	}

	return absPath, err
}

func ext(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	path := data.String(args.Get(0))

	return filepath.Ext(path), nil
}

func dir(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	path := data.String(args.Get(0))

	return filepath.Dir(path), nil
}

func clean(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	path := data.String(args.Get(0))

	return filepath.Clean(path), nil
}
