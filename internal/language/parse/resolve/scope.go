package resolve

import "github.com/tucats/ego/internal/language/parse/ast"

// ScopeKind categorizes a Scope by the kind of AST construct that opened it.
type ScopeKind int

const (
	// ScopeInvalid is the zero value and marks an uninitialized Scope.
	ScopeInvalid ScopeKind = iota

	// ScopeFile is the single root scope for a compilation unit: top-level
	// funcs, vars, consts, types, and imports.
	ScopeFile

	// ScopeFunction is the scope opened for a function/method body or a
	// function literal: parameters, named returns, and the receiver live
	// here, one level outside the body's own top-level ScopeBlock.
	ScopeFunction

	// ScopeBlock is the scope opened for a brace-delimited block ("{ ... }")
	// or an equivalent lexical unit that is not itself a whole function body
	// (a for-loop's init/cond/post clause, a switch's init clause, or one
	// case/default clause body).
	ScopeBlock
)

var scopeKindNames = map[ScopeKind]string{
	ScopeInvalid:  "invalid",
	ScopeFile:     "file",
	ScopeFunction: "function",
	ScopeBlock:    "block",
}

func (k ScopeKind) String() string {
	if name, ok := scopeKindNames[k]; ok {
		return name
	}

	return "ScopeKind(?)"
}

// Scope is one lexical block: a mapping from name to Symbol, plus the parent
// chain a name lookup walks when it is not found locally.
type Scope struct {
	// Kind identifies what kind of construct opened this scope.
	Kind ScopeKind

	// Node is the AST node that opened this scope: an *ast.File,
	// *ast.FuncDecl, *ast.FuncLit, *ast.Block, *ast.ForStmt, *ast.SwitchStmt,
	// or *ast.CaseClause.
	Node ast.Node

	// Parent is the enclosing scope, nil only for the root ScopeFile scope.
	Parent *Scope

	// Children lists scopes opened directly inside this one, in source order.
	Children []*Scope

	// Symbols maps a declared name to its Symbol, for O(1) lookup. Only the
	// most recent declaration of a given name within this exact scope is
	// kept -- a same-scope redeclaration (illegal in real Ego/Go outside a
	// few special cases the current compiler already handles, e.g. loop-body
	// idempotent decls) simply replaces the previous entry here.
	Symbols map[string]*Symbol

	// Order lists this scope's own Symbols in declaration order, for
	// deterministic iteration (diagnostics, dumps, tests).
	Order []*Symbol

	// FuncBoundary is the nearest enclosing ScopeFunction scope: itself, if
	// this scope's Kind is ScopeFunction, otherwise its parent's
	// FuncBoundary. Nil for the ScopeFile root and any scope not nested
	// inside a function. Used by walk.go's capture detection: a reference
	// resolves to a symbol captured by a closure exactly when the
	// referencing FuncLit's own FuncBoundary differs from the symbol's
	// Scope.FuncBoundary.
	FuncBoundary *Scope
}

// newScope creates a child scope of parent (nil for the root ScopeFile scope)
// and links it into parent.Children.
func newScope(kind ScopeKind, node ast.Node, parent *Scope) *Scope {
	s := &Scope{
		Kind:    kind,
		Node:    node,
		Parent:  parent,
		Symbols: map[string]*Symbol{},
	}

	if kind == ScopeFunction {
		s.FuncBoundary = s
	} else if parent != nil {
		s.FuncBoundary = parent.FuncBoundary
	}

	if parent != nil {
		parent.Children = append(parent.Children, s)
	}

	return s
}

// declare records sym as declared in this scope, replacing any earlier
// symbol of the same name declared directly in this scope.
func (s *Scope) declare(sym *Symbol) {
	sym.Scope = s
	s.Symbols[sym.Name] = sym
	s.Order = append(s.Order, sym)
}

// lookup resolves name by searching this scope and then each ancestor in
// turn, returning the first match or nil if the name is not declared
// anywhere in the chain.
func (s *Scope) lookup(name string) *Symbol {
	for cur := s; cur != nil; cur = cur.Parent {
		if sym, ok := cur.Symbols[name]; ok {
			return sym
		}
	}

	return nil
}
