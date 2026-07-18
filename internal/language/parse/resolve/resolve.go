// Package resolve performs a best-effort, single-file static analysis of an
// Ego AST (github.com/tucats/ego/internal/language/parse/ast): it builds a
// symbol table with lexical scope information, resolves identifier
// references, classifies each function-local symbol's register/heap storage
// suitability based on closure capture, and reports unused, undefined, and
// used-before-defined names.
//
// This mirrors go/types' Info-attached-to-ast pattern: Resolve returns an
// *Info side-table (Defs/Uses maps keyed by *ast.Ident, a Scopes map keyed by
// ast.Node) rather than annotating the ast package's node types directly, so
// ast/ stays free of semantic concerns (per its own "open interface" design
// goal) and this package stays a read-only consumer of ast, exactly like the
// existing format package.
//
// # Known limitations
//
// This is intentionally a first step, not a replacement for the compiler's
// own, fully context-aware name resolution:
//
//   - Single *ast.File only. There is no cross-file package symbol merging,
//     so a top-level func/type/var/const/import is never reported
//     UnusedSymbol -- another file in the same package may use it, and this
//     package cannot know.
//   - No knowledge of compiler settings that change what names are in scope
//     (ego.compiler.import, ego.compiler.extensions) or of loaded runtime
//     packages. A small, deliberately non-exhaustive builtin allow-list
//     (builtins.go) is used instead -- see its doc comment.
//   - No type-system knowledge. Symbol.Type is a raw syntactic ast.Node
//     (present only when written explicitly, or trivially derivable from a
//     literal), never a resolved/inferred type.
//   - Type-position nodes (a type cast's target, a var's declared type, a
//     struct field's type, a type assertion's target) are never resolved as
//     identifier references, even when spelled as a bare name -- the parser
//     represents them as ast.NamedType/ast.PrimitiveType/etc., not
//     ast.Ident, so they do not participate in Defs/Uses at all.
//   - Directive arguments (DirectiveStmt) are only walked for directives
//     whose entire grammar is a single optional expression -- @assert,
//     @error, @fail, @status, @symbols (see the parse package's
//     directivesWithExprArgs) -- since only those get real Args nodes at
//     parse time; every other directive (@capture, @compile, @template,
//     @global, @log, @define, @package, @json, @text, ...) still carries
//     only raw token spellings (DirectiveStmt.RawArgs), so identifiers
//     referenced inside one of those are invisible to this package. This
//     was the single largest source of noise found calibrating against the
//     real tests/ and lib/packages/ corpus (see docs/SYNTAX.md section 12
//     for each directive's argument grammar) before the parse package
//     gained expression-argument parsing for the directives above.
//   - A directive's Args (where present, per the point above) are parsed by
//     re-tokenizing DirectiveStmt.RawArgs in a fresh, isolated Parser (see
//     parseDirectiveExprArgs in the parse package), not by extending the
//     position of the original source scan. Any diagnostic anchored to an
//     identifier inside such an Args expression therefore reports a
//     position relative to that synthetic re-parse (typically line 1), not
//     the identifier's true location in the source file.
//   - UsedBeforeDefined is decided from a single, whole-activation
//     (function body, or the whole file for a bare fragment) textual
//     pre-scan of declared names (see walk.go's collectLocalDeclNames), not
//     a scope-precise one. Two unrelated sibling scopes that happen to
//     reuse the same local name -- most visibly, two separate @test blocks
//     in one bare fragment each declaring their own "x" or "err" -- can
//     cause a genuinely-unresolvable reference in one to be labeled
//     UsedBeforeDefined (pointing at the other's unrelated declaration)
//     rather than UndefinedSymbol. The reference is still correctly
//     reported as a problem either way; only the specific diagnostic Kind
//     can be imprecise in this case.
//   - This package cannot resolve a bare type name used as a first-class
//     value with no trailing cast/composite suffix (the "compare against a
//     type" idiom seen in this codebase as
//     `reflect.Type(x) == interface{}` / `... == error`): the underlying
//     parse package's expression-atom dispatch does not recognize a type
//     keyword in that position at all (confirmed independent of this
//     package, and independent of directive-argument parsing, by parsing
//     `x := reflect.Type(1) == interface{}` directly with
//     parse.ParseStatements) and instead emits a bare *ast.Ident whose Name
//     is the type's own spelling (e.g. "interface{}"), which this package
//     then correctly, but unhelpfully, reports as UndefinedSymbol. Fixing
//     this is a parse-package atom-dispatch gap, out of this package's
//     scope.
//   - Throughout, the design principle is to prefer under-reporting to false
//     positives: an ambiguous or unrecognized construct is left unresolved
//     silently rather than guessed at and potentially misdiagnosed.
package resolve

import "github.com/tucats/ego/internal/language/parse/ast"

// Info is the result of Resolve for one *ast.File.
type Info struct {
	// Root is the file-level (package) scope.
	Root *Scope

	// Scopes maps each AST node that opens a scope (*ast.File, *ast.FuncDecl,
	// *ast.FuncLit, *ast.Block, and the *ast.ForStmt/*ast.SwitchStmt/
	// *ast.CaseClause/*ast.TryStmt/*ast.IfStmt wrapping scopes described in
	// scope.go) to the Scope it opened.
	Scopes map[ast.Node]*Scope

	// Defs maps each identifier that is itself a declaring occurrence to the
	// Symbol it declared.
	Defs map[*ast.Ident]*Symbol

	// Uses maps each identifier that is itself a resolved reference to the
	// Symbol it refers to.
	Uses map[*ast.Ident]*Symbol

	// Symbols lists every declared Symbol, across every scope, in
	// declaration order.
	Symbols []*Symbol
}

// newInfo creates an empty Info with its maps initialized.
func newInfo() *Info {
	return &Info{
		Scopes: map[ast.Node]*Scope{},
		Defs:   map[*ast.Ident]*Symbol{},
		Uses:   map[*ast.Ident]*Symbol{},
	}
}

// declare records sym as declared in scope, appends it to the flat Symbols
// list, and -- when sym has a defining identifier -- records the defining
// occurrence in Defs. Every declaration site in this package (collect.go and
// walk.go) goes through this single method so Symbols/Defs stay complete and
// consistent.
func (info *Info) declare(scope *Scope, sym *Symbol) {
	scope.declare(sym)
	info.Symbols = append(info.Symbols, sym)

	if sym.DeclIdent != nil {
		info.Defs[sym.DeclIdent] = sym
	}
}

// registerScope records scope in info.Scopes, keyed by the node that opened
// it.
func (info *Info) registerScope(scope *Scope) {
	if scope.Node != nil {
		info.Scopes[scope.Node] = scope
	}
}

// Resolve analyzes file and returns its symbol/scope information together
// with any diagnostics found. See the package doc for scope and limitations.
func Resolve(file *ast.File) (*Info, []Diagnostic) {
	info := newInfo()
	info.Root = newScope(ScopeFile, file, nil)
	info.registerScope(info.Root)

	collectFileScope(file, info)

	w := &walker{info: info}

	if file.Bare {
		// A bare fragment (REPL input, an @test block) behaves as a single
		// implicit function activation, not a package: give the root scope
		// its own FuncBoundary so its locals are classified Register/Heap
		// like any other function's, not PackageGlobal, and so
		// used-before-defined tracking applies to it.
		info.Root.FuncBoundary = info.Root

		w.pushFuncLocals(file)

		for _, decl := range file.Decls {
			w.walkStmt(info.Root, decl)
		}

		w.popFuncLocals()
		classifyStorageForFunction(info.Root)
	} else {
		resolveFileInitializers(w, file)

		for _, decl := range file.Decls {
			if d, ok := decl.(*ast.FuncDecl); ok {
				w.walkFuncDecl(info.Root, d)
			}
		}
	}

	return info, w.diags
}
