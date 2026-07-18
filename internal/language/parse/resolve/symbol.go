package resolve

import "github.com/tucats/ego/internal/language/parse/ast"

// SymbolKind categorizes a declared name by the kind of declaration that
// introduced it.
type SymbolKind int

const (
	// KindInvalid is the zero value and marks an uninitialized Symbol.
	KindInvalid SymbolKind = iota
	KindVar
	KindConst
	KindParam
	KindNamedReturn
	KindReceiver
	KindFunc
	KindType
	KindImport
	KindLabel
)

var symbolKindNames = map[SymbolKind]string{
	KindInvalid:     "invalid",
	KindVar:         "var",
	KindConst:       "const",
	KindParam:       "param",
	KindNamedReturn: "namedReturn",
	KindReceiver:    "receiver",
	KindFunc:        "func",
	KindType:        "type",
	KindImport:      "import",
	KindLabel:       "label",
}

func (k SymbolKind) String() string {
	if name, ok := symbolKindNames[k]; ok {
		return name
	}

	return "SymbolKind(?)"
}

// StorageHint classifies where a future compiler could plausibly store a
// symbol's value at runtime, based purely on lexical/capture analysis (see
// walk.go's capture detection). It deliberately mirrors the register-vs-heap
// distinction docs/SLOTS.md already draws for the existing token-based
// compiler, but computed per-symbol instead of per-function.
type StorageHint int

const (
	// StorageUnknown is the zero value: a symbol whose storage class has not
	// been classified (e.g. it is not function-local at all, such as a Func
	// or Type symbol).
	StorageUnknown StorageHint = iota

	// StorageRegister marks a function-local symbol (var, const, param, named
	// return, receiver) that is never referenced from within a nested
	// function literal, so its lifetime is provably bounded by its own
	// call activation.
	StorageRegister

	// StorageHeap marks a function-local symbol that is referenced from
	// within at least one nested function literal in its scope, so it may
	// outlive its own call activation and cannot be safely stack/register
	// allocated.
	StorageHeap

	// StoragePackageGlobal marks a symbol declared at file/package scope.
	StoragePackageGlobal
)

var storageHintNames = map[StorageHint]string{
	StorageUnknown:       "unknown",
	StorageRegister:      "register",
	StorageHeap:          "heap",
	StoragePackageGlobal: "packageGlobal",
}

func (h StorageHint) String() string {
	if name, ok := storageHintNames[h]; ok {
		return name
	}

	return "StorageHint(?)"
}

// Symbol represents one declared name: a variable, constant, parameter, named
// return, receiver, function, type, import, or label. It is a compile-time
// analysis artifact, distinct from (and not to be confused with) the runtime
// internal/language/symbols.SymbolAttribute, which records a live value's
// storage slot during actual execution.
type Symbol struct {
	// Name is the declared identifier spelling.
	Name string

	// Kind is the flavor of declaration that introduced this symbol.
	Kind SymbolKind

	// DeclNode is the declaration node (e.g. the *ast.VarSpec, *ast.Param, or
	// *ast.FuncDecl) that introduced this symbol. It is never nil.
	DeclNode ast.Node

	// DeclIdent is the *ast.Ident of the defining occurrence, when the
	// declaration syntax provides one. It is nil for the rare case of a
	// declaration with no dedicated Ident node -- currently only
	// TryStmt.CatchVar, which is a bare string in the AST.
	DeclIdent *ast.Ident

	// Type is the syntactic type node from the declaration, when one was
	// written explicitly or is trivially derivable from a literal RHS (see
	// walk.go's inferLiteralType). Nil when no type could be determined this
	// way -- this package does not perform general expression-type
	// inference (see package doc).
	Type ast.Node

	// Scope is the lexical scope this symbol was declared into.
	Scope *Scope

	// Uses lists every identifier occurrence that resolved to this symbol in
	// a read (or read+write, e.g. IncDecStmt/compound-assign) position. An
	// empty Uses slice at scope-exit is what drives the UnusedSymbol
	// diagnostic (see walk.go).
	Uses []*ast.Ident

	// Captured is true if some nested function literal within this symbol's
	// declaring function references it. See the package doc and walk.go for
	// the capture-detection algorithm.
	Captured bool

	// CapturedBy lists the *ast.FuncLit nodes whose bodies reference this
	// symbol from an enclosing function scope.
	CapturedBy []ast.Node

	// Storage is the register/heap/package-global classification derived
	// from Captured (see StorageHint).
	Storage StorageHint
}

// Pos returns the position of the symbol's defining occurrence, preferring
// DeclIdent when present and falling back to DeclNode otherwise.
func (s *Symbol) Pos() ast.Position {
	if s.DeclIdent != nil {
		return s.DeclIdent.Pos()
	}

	if s.DeclNode != nil {
		return s.DeclNode.Pos()
	}

	return ast.Position{}
}
