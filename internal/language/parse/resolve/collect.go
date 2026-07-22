package resolve

import "github.com/tucats/ego/internal/language/parse/ast"

// collectFileScope is Pass 1: it registers every top-level declaration into
// info.Root, in whatever order they appear in file.Decls. Registering all
// names up front (rather than sequentially, as Pass 2 does for function
// bodies) is what lets mutually-recursive top-level functions/types resolve
// regardless of source order -- matching the current compiler's
// resolveExternalSymbol, which defers "unknown symbol" reporting for exactly
// this reason.
//
// A bare fragment (file.Bare == true, produced by parse.ParseStatements for
// REPL input / @test blocks) has no such two-phase structure: executable
// statements and declarations are interleaved at the top level and must be
// resolved strictly in source order, exactly like a function body. For a
// bare file, collectFileScope does nothing, and Resolve (resolve.go) walks
// file.Decls sequentially against the (initially empty) root scope instead.
func collectFileScope(file *ast.File, info *Info) {
	if file.Bare {
		return
	}

	root := info.Root

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			registerFuncDecl(info, root, d)
		case *ast.VarDecl:
			for _, spec := range d.Specs {
				registerVarSpec(info, root, spec, KindVar)
			}
		case *ast.ConstDecl:
			for _, spec := range d.Specs {
				registerConstSpec(info, root, spec)
			}
		case *ast.TypeDecl:
			registerTypeDecl(info, root, d)
		case *ast.ImportDecl:
			for _, spec := range d.Specs {
				registerImportSpec(info, root, spec)
			}
		}
	}
}

// registerFuncDecl declares d's name in scope, unless d is a method (has a
// receiver). Methods are not registered as bare-callable file-scope names --
// matching Go, they are only reachable as receiver.Method(), and this
// package does not perform the type-based method-set resolution that would
// be needed to track such references (see package doc). The method's body is
// still walked in Pass 2 regardless of whether a symbol was registered here.
func registerFuncDecl(info *Info, scope *Scope, d *ast.FuncDecl) {
	if d.Recv != nil || d.Name == nil || d.Name.Name == "" {
		return
	}

	info.declare(scope, &Symbol{
		Name:      d.Name.Name,
		Kind:      KindFunc,
		DeclNode:  d,
		DeclIdent: d.Name,
	})
}

// registerVarSpec declares each name in spec into scope with the given kind
// (KindVar at file scope; walk.go reuses this for function-body "var"
// statements too). The syntactic type is used when written explicitly;
// otherwise, when Names and Values correspond one-to-one ("var a, b = 1,
// 2"), a type is inferred per-name from a literal Value the same way ":="
// does (see inferLiteralType) -- left nil for the ambiguous multi-return
// form ("var a, b = f()"). A file-scope declaration is classified
// StoragePackageGlobal immediately, since walk.go's
// classifyStorageForFunction (which assigns Storage for every other scope,
// once its owning function's body has been fully walked) never visits the
// file scope for a non-bare file.
func registerVarSpec(info *Info, scope *Scope, spec *ast.VarSpec, kind SymbolKind) {
	positional := len(spec.Names) == len(spec.Values)

	for i, name := range spec.Names {
		if name == nil || name.Name == "" || name.Name == "_" {
			continue
		}

		typ := spec.Type
		if typ == nil && positional {
			typ = inferLiteralType(spec.Values[i])
		}

		info.declare(scope, &Symbol{
			Name:      name.Name,
			Kind:      kind,
			DeclNode:  spec,
			DeclIdent: name,
			Type:      typ,
			Storage:   fileScopeStorage(scope),
		})
	}
}

// registerConstSpec declares a const's name into scope. It uses the explicit
// type of a typed constant ("Name Type = expr") when present, and otherwise
// infers the type from a literal Value the same way ":="/"var" do (see
// inferLiteralType).
func registerConstSpec(info *Info, scope *Scope, spec *ast.ConstSpec) {
	if spec.Name == nil || spec.Name.Name == "" {
		return
	}

	typ := spec.Type
	if typ == nil {
		typ = inferLiteralType(spec.Value)
	}

	info.declare(scope, &Symbol{
		Name:      spec.Name.Name,
		Kind:      KindConst,
		DeclNode:  spec,
		DeclIdent: spec.Name,
		Type:      typ,
		Storage:   fileScopeStorage(scope),
	})
}

// fileScopeStorage returns StoragePackageGlobal for a file-scope
// declaration, or the zero StorageUnknown for any other scope (function
// bodies are classified later, in bulk, by walk.go's
// classifyStorageForFunction once their whole body has been walked).
func fileScopeStorage(scope *Scope) StorageHint {
	if scope.Kind == ScopeFile {
		return StoragePackageGlobal
	}

	return StorageUnknown
}

// registerTypeDecl declares a type name into scope. The type's own body
// (struct fields, interface methods) is not walked for symbol references --
// field/method types are ast.NamedType/ast.QualifiedType nodes, not Idents,
// and resolving them would require the general type-name resolution this
// package intentionally defers (see package doc).
func registerTypeDecl(info *Info, scope *Scope, d *ast.TypeDecl) {
	if d.Name == nil || d.Name.Name == "" {
		return
	}

	info.declare(scope, &Symbol{
		Name:      d.Name.Name,
		Kind:      KindType,
		DeclNode:  d,
		DeclIdent: d.Name,
		Type:      d.Type,
	})
}

// registerImportSpec declares an import's local name (its alias, or the last
// path segment when no alias is given) into scope.
func registerImportSpec(info *Info, scope *Scope, spec *ast.ImportSpec) {
	name := importLocalName(spec)
	if name == "" {
		return
	}

	info.declare(scope, &Symbol{
		Name:     name,
		Kind:     KindImport,
		DeclNode: spec,
	})
}

// importLocalName returns the identifier a file uses to refer to an import:
// its alias when one was given, otherwise the last "/"-separated segment of
// its path.
func importLocalName(spec *ast.ImportSpec) string {
	if spec.Alias != "" {
		return spec.Alias
	}

	path := spec.Path

	if idx := lastSlash(path); idx >= 0 {
		return path[idx+1:]
	}

	return path
}

// lastSlash returns the index of the last "/" in s, or -1 if none.
func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}

	return -1
}

// resolveFileInitializers walks the initializer expressions of every
// top-level var/const spec against the fully-populated root scope, after
// every top-level name has already been registered by collectFileScope. This
// gives file-scope initializers the same forward-reference tolerance as
// function/type references: order-independent, since package-level
// initializers are conceptually a single mutually-visible group here (this
// package does not attempt Go's true dependency-ordering analysis for
// package-level initialization -- see package doc).
func resolveFileInitializers(w *walker, file *ast.File) {
	if file.Bare {
		return
	}

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.VarDecl:
			for _, spec := range d.Specs {
				for _, v := range spec.Values {
					w.resolveExpr(w.info.Root, v)
				}
			}
		case *ast.ConstDecl:
			for _, spec := range d.Specs {
				w.resolveExpr(w.info.Root, spec.Value)
			}
		}
	}
}
