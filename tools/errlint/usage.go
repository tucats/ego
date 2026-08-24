package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// findUsedSymbols walks root recursively and returns the set of identifier
// names referenced anywhere in the .go files it finds, skipping the files
// named in skip (given as absolute paths) along with .git and vendor
// directories.
//
// An identifier name is recorded whether it appears as a bare reference
// (ErrFoo) or as the selector half of a package-qualified one
// (errors.ErrFoo), since both are represented as *ast.Ident nodes by the
// parser; a plain, untyped identifier scan is a deliberate simplification
// that trades perfect precision (it would, in principle, treat an unrelated
// identifier that happens to share an error symbol's name as a "use") for
// not having to resolve imports and types, which is a reasonable trade for
// a project convention where error symbols use a distinct Err* naming
// pattern.
func findUsedSymbols(root string, skip map[string]bool) (map[string]bool, error) {
	used := map[string]bool{}
	fset := token.NewFileSet()

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			switch info.Name() {
			case ".git", "vendor":
				return filepath.SkipDir
			}

			return nil
		}

		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		abs, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolving %s: %w", path, err)
		}

		if skip[abs] {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}

		ast.Inspect(file, func(n ast.Node) bool {
			if ident, ok := n.(*ast.Ident); ok {
				used[ident.Name] = true
			}

			return true
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	return used, nil
}
