package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
)

// errorDef is one "var Symbol = Message("key")" (or grouped equivalent)
// declaration found in an --errors file.
type errorDef struct {
	Symbol string
	Key    string
	File   string
	Line   int
}

// extractErrorDefs parses a Go source file and returns one errorDef for
// every top-level "var" declaration whose initializer is a call to
// Message(...) or <pkg>.Message(...) with a single string literal
// argument. This covers both individual declarations
//
//	var ErrFoo = Message("foo")
//
// and grouped ones
//
//	var (
//	        ErrFoo = Message("foo")
//	        ErrBar = Message("bar")
//	)
//
// Declarations that don't match this shape (no initializer, a call to
// something other than Message, a non-literal argument, and so on) are
// silently skipped rather than treated as errors, since an --errors file
// may reasonably contain other kinds of package-level state.
func extractErrorDefs(path string) ([]errorDef, error) {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	var defs []errorDef

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}

		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			for i, name := range valueSpec.Names {
				if i >= len(valueSpec.Values) {
					continue
				}

				key, ok := messageCallKey(valueSpec.Values[i])
				if !ok {
					continue
				}

				pos := fset.Position(name.Pos())

				defs = append(defs, errorDef{
					Symbol: name.Name,
					Key:    key,
					File:   path,
					Line:   pos.Line,
				})
			}
		}
	}

	return defs, nil
}

// messageCallKey reports whether expr is a call to a function named
// Message (either a bare identifier or the selector of a package-qualified
// call, such as errors.Message) with a single string-literal argument, and
// if so, returns that literal's decoded value.
func messageCallKey(expr ast.Expr) (string, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return "", false
	}

	var name string

	switch fn := call.Fun.(type) {
	case *ast.Ident:
		name = fn.Name
	case *ast.SelectorExpr:
		name = fn.Sel.Name
	default:
		return "", false
	}

	if name != "Message" {
		return "", false
	}

	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}

	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}

	return value, true
}
