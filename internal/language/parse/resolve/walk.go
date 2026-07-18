package resolve

import (
	"fmt"

	"github.com/tucats/ego/internal/language/parse/ast"
)

// walker carries the mutable state threaded through Pass 2 (the sequential
// body walk).
type walker struct {
	info  *Info
	diags []Diagnostic

	// funcLocals is a stack, one entry per function activation currently
	// being walked (the outer file, for a bare fragment; each *ast.FuncDecl/
	// *ast.FuncLit otherwise). Each entry maps every name that ast.Walk
	// finds declared anywhere in that function's own body (see
	// collectLocalDeclNames) to its declaration position. It exists solely
	// to distinguish, for an otherwise-unresolved reference, a genuine
	// forward reference to a same-function local (UsedBeforeDefined) from a
	// truly undefined name (UndefinedSymbol).
	funcLocals []map[string]ast.Position
}

// pushFuncLocals starts tracking a new function activation, pre-scanning
// root (an *ast.Block function body, or the *ast.File itself for a bare
// fragment) for every name it declares.
func (w *walker) pushFuncLocals(root ast.Node) {
	names := map[string]ast.Position{}

	if root != nil {
		collectLocalDeclNames(root, names)
	}

	w.funcLocals = append(w.funcLocals, names)
}

// popFuncLocals ends tracking for the innermost function activation.
func (w *walker) popFuncLocals() {
	w.funcLocals = w.funcLocals[:len(w.funcLocals)-1]
}

// currentFuncLocals returns the innermost function activation's pre-scanned
// local-declaration set, or nil if no function activation is active (e.g.
// while resolving a top-level, non-bare file's var/const initializers).
func (w *walker) currentFuncLocals() map[string]ast.Position {
	if len(w.funcLocals) == 0 {
		return nil
	}

	return w.funcLocals[len(w.funcLocals)-1]
}

// collectLocalDeclNames walks root (without descending into nested function
// literal bodies -- each has its own, independently pre-scanned, local
// namespace) recording every name declared by a ":=" or "var" statement
// anywhere inside it.
func collectLocalDeclNames(root ast.Node, into map[string]ast.Position) {
	ast.Walk(root, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.AssignStmt:
			if v.Op == ":=" {
				for _, lhs := range v.Lhs {
					recordLocalDeclName(lhs, into)
				}
			}
		case *ast.VarSpec:
			for _, name := range v.Names {
				recordLocalDeclName(name, into)
			}
		}

		return true
	})
}

// recordLocalDeclName records n's name (when n is a non-discard *ast.Ident)
// in into, keeping the first position seen for a given name.
func recordLocalDeclName(n ast.Node, into map[string]ast.Position) {
	id, ok := n.(*ast.Ident)
	if !ok || id.Name == "" || id.Name == "_" {
		return
	}

	if _, exists := into[id.Name]; !exists {
		into[id.Name] = id.Pos()
	}
}

// walkStmtsIn walks each statement in stmts, in order, against scope.
func (w *walker) walkStmtsIn(scope *Scope, stmts []ast.Node) {
	for _, stmt := range stmts {
		w.walkStmt(scope, stmt)
	}
}

// walkStmt dispatches a single statement node to its specific handling.
// Constructs that open their own lexical scope create and finalize a child
// Scope; everything else resolves its expression operands against scope and,
// for a declaring statement, declares the new name(s) into scope.
func (w *walker) walkStmt(scope *Scope, n ast.Node) {
	if n == nil {
		return
	}

	switch v := n.(type) {
	case *ast.Block:
		blockScope := newScope(ScopeBlock, v, scope)
		w.info.registerScope(blockScope)
		w.walkStmtsIn(blockScope, v.Stmts)
		w.finalizeScope(blockScope)

	case *ast.AssignStmt:
		w.walkAssign(scope, v)

	case *ast.VarDecl:
		for _, spec := range v.Specs {
			for _, val := range spec.Values {
				w.resolveExpr(scope, val)
			}

			registerVarSpec(w.info, scope, spec, KindVar)
		}

	case *ast.ConstDecl:
		for _, spec := range v.Specs {
			w.resolveExpr(scope, spec.Value)
			registerConstSpec(w.info, scope, spec)
		}

	case *ast.TypeDecl:
		registerTypeDecl(w.info, scope, v)

	case *ast.IncDecStmt:
		w.resolveExpr(scope, v.X)

	case *ast.SendStmt:
		w.resolveExpr(scope, v.Chan)
		w.resolveExpr(scope, v.Value)

	case *ast.ReturnStmt:
		for _, r := range v.Results {
			w.resolveExpr(scope, r)
		}

	case *ast.ExprStmt:
		w.resolveExpr(scope, v.X)

	case *ast.CallStmt:
		w.resolveExpr(scope, v.Call)

	case *ast.DeferStmt:
		w.resolveExpr(scope, v.Call)

	case *ast.GoStmt:
		w.resolveExpr(scope, v.Call)

	case *ast.PanicStmt:
		w.resolveExpr(scope, v.Arg)

	case *ast.ThrowStmt:
		w.resolveExpr(scope, v.X)

	case *ast.PrintStmt:
		for _, a := range v.Args {
			w.resolveExpr(scope, a)
		}

	case *ast.ExitStmt:
		w.resolveExpr(scope, v.Code)

	case *ast.IfStmt:
		w.walkIf(scope, v)

	case *ast.ForStmt:
		w.walkFor(scope, v)

	case *ast.SwitchStmt:
		w.walkSwitch(scope, v)

	case *ast.TryStmt:
		w.walkTry(scope, v)

	case *ast.LabeledStmt:
		w.declareLabel(scope, v)
		w.walkStmt(scope, v.Stmt)

	case *ast.DirectiveStmt:
		for _, a := range v.Args {
			w.resolveExpr(scope, a)
		}

	case *ast.FuncDecl:
		// A named function declaration nested inside a block -- legal Ego
		// syntax (see walkFuncDecl's doc comment), most commonly a small
		// helper like "near" declared inside a bare @test fragment's block.
		w.walkFuncDecl(scope, v)

	case *ast.BreakStmt, *ast.ContinueStmt, *ast.EmptyStmt:
		// No symbol references.

	default:
		// Keeps this switch from needing to be exhaustive as the grammar
		// evolves: fall back to generic expression resolution.
		w.resolveExpr(scope, n)
	}
}

// walkAssign handles both the declaring (":=") and non-declaring ("=" and
// the compound "+=", "-=", "*=", "/=") forms of AssignStmt. The Rhs is always
// resolved before any new Lhs name becomes visible, so a shadowing
// declaration like "x := x + 1" resolves the Rhs "x" to the outer binding --
// matching the documented pending-declaration semantics in docs/SLOTS.md.
func (w *walker) walkAssign(scope *Scope, a *ast.AssignStmt) {
	for _, rhs := range a.Rhs {
		w.resolveExpr(scope, rhs)
	}

	if a.Op == ":=" {
		// A type can only be inferred from a positionally-matching literal
		// RHS ("x := 5"), not from the multi-return call form ("a, b :=
		// f()"), where no such correspondence exists.
		positional := len(a.Lhs) == len(a.Rhs)

		for i, lhs := range a.Lhs {
			id, ok := lhs.(*ast.Ident)
			if !ok || id.Name == "" || id.Name == "_" {
				continue
			}

			sym := &Symbol{Name: id.Name, Kind: KindVar, DeclNode: a, DeclIdent: id}
			if positional {
				sym.Type = inferLiteralType(a.Rhs[i])
			}

			w.info.declare(scope, sym)
		}

		return
	}

	// A compound operator reads the old value before writing the new one, so
	// it counts as a use; a plain "=" does not -- matching Go's own
	// "declared and not used" rule, which a bare assignment alone never
	// satisfies.
	readWrite := a.Op != "="

	for _, lhs := range a.Lhs {
		id, ok := lhs.(*ast.Ident)
		if !ok {
			// e.g. "arr[i] = x" or "obj.Field = x" -- the sub-expressions
			// (arr, i, obj) are genuine reads.
			w.resolveExpr(scope, lhs)

			continue
		}

		if readWrite {
			w.resolveExpr(scope, id)
		} else {
			w.resolveWriteOnly(scope, id)
		}
	}
}

// resolveWriteOnly resolves id for UndefinedSymbol-checking purposes only:
// it does not record a use and does not mark the symbol as read. See
// walkAssign.
func (w *walker) resolveWriteOnly(scope *Scope, id *ast.Ident) {
	if id == nil || id.Name == "" || id.Name == "_" {
		return
	}

	if sym := scope.lookup(id.Name); sym == nil {
		w.reportUnresolved(scope, id)
	}
}

// literalTypeNames maps a BasicLit's LitKind to the Ego primitive type name
// its value unambiguously has. LitNil and LitInvalid are intentionally
// absent: nil's static type depends entirely on its context, and there is no
// type to attach to an invalid literal.
var literalTypeNames = map[ast.LitKind]string{
	ast.LitInt:       "int",
	ast.LitFloat:     "float64",
	ast.LitImaginary: "complex128",
	ast.LitString:    "string",
	ast.LitRune:      "int32",
	ast.LitBool:      "bool",
}

// inferLiteralType returns a synthetic *ast.PrimitiveType for rhs when it is
// directly a literal of a kind with an unambiguous Ego type, or nil
// otherwise. This is the full extent of this package's "type inference" --
// no general expression-type inference (binary operations, calls, composite
// literals) is attempted; see the package doc's "syntactic only" scope.
func inferLiteralType(rhs ast.Node) ast.Node {
	lit, ok := rhs.(*ast.BasicLit)
	if !ok {
		return nil
	}

	name, ok := literalTypeNames[lit.LitKind]
	if !ok {
		return nil
	}

	return &ast.PrimitiveType{Name: name}
}

// walkIf gives an if statement with an init clause its own wrapping scope,
// covering Init, Cond, Body, and Else, so the init variable is visible
// throughout the if/else chain but does not leak into the surrounding block.
// (The current token-based compiler does not do this -- compileIf only
// emits a runtime PushScope/PopScope pair around the init statement without
// a matching compile-time c.scopes entry -- but nothing prevents this AST
// analysis from being more precise here; see the package doc's framing of
// this as an area where the AST version can improve on, not just replicate,
// the existing compiler.)
func (w *walker) walkIf(scope *Scope, s *ast.IfStmt) {
	ifScope := scope

	if s.Init != nil {
		ifScope = newScope(ScopeBlock, s, scope)
		w.info.registerScope(ifScope)
		w.walkStmt(ifScope, s.Init)
	}

	w.resolveExpr(ifScope, s.Cond)
	w.walkStmt(ifScope, s.Body)

	if s.Else != nil {
		w.walkStmt(ifScope, s.Else)
	}

	if s.Init != nil {
		w.finalizeScope(ifScope)
	}
}

// walkFor handles all four for-loop forms (infinite, conditional,
// three-clause, range). A wrapping scope is opened only when the loop
// declares its own variables (Init or a ":="-defined range Key/Value), so
// that they are visible across Cond/Post/Body but not beyond the loop.
func (w *walker) walkFor(scope *Scope, s *ast.ForStmt) {
	loopScope := scope
	opensScope := s.Init != nil || s.Key != nil

	if opensScope {
		loopScope = newScope(ScopeBlock, s, scope)
		w.info.registerScope(loopScope)
	}

	if s.Init != nil {
		w.walkStmt(loopScope, s.Init)
	}

	if s.Cond != nil {
		w.resolveExpr(loopScope, s.Cond)
	}

	if s.Range != nil {
		w.resolveExpr(loopScope, s.Range)
	}

	if s.Define {
		w.declareLoopVar(loopScope, s.Key)
		w.declareLoopVar(loopScope, s.Value)
	} else {
		w.resolveExpr(loopScope, s.Key)
		w.resolveExpr(loopScope, s.Value)
	}

	if s.Post != nil {
		w.walkStmt(loopScope, s.Post)
	}

	if s.Body != nil {
		w.walkStmt(loopScope, s.Body)
	}

	if opensScope {
		w.finalizeScope(loopScope)
	}
}

// declareLoopVar declares a range loop's ":="-introduced Key or Value
// variable, when n is a non-discard *ast.Ident.
func (w *walker) declareLoopVar(scope *Scope, n ast.Node) {
	id, ok := n.(*ast.Ident)
	if !ok || id.Name == "" || id.Name == "_" {
		return
	}

	w.info.declare(scope, &Symbol{Name: id.Name, Kind: KindVar, DeclNode: id, DeclIdent: id})
}

// walkSwitch opens a wrapping scope for the init clause (when present, so
// its variable is visible to every case) and one further scope per
// case/default clause -- matching the current compiler's own switch.go
// scoping exactly (see docs comment there): a variable declared in one case
// body is not visible to any other case, including across "fallthrough".
func (w *walker) walkSwitch(scope *Scope, s *ast.SwitchStmt) {
	switchScope := scope

	if s.Init != nil {
		switchScope = newScope(ScopeBlock, s, scope)
		w.info.registerScope(switchScope)
		w.walkStmt(switchScope, s.Init)
	}

	if s.Tag != nil {
		w.resolveExpr(switchScope, s.Tag)
	}

	for _, cc := range s.Body {
		caseScope := newScope(ScopeBlock, cc, switchScope)
		w.info.registerScope(caseScope)

		for _, e := range cc.Exprs {
			w.resolveExpr(caseScope, e)
		}

		w.walkStmtsIn(caseScope, cc.Body)
		w.finalizeScope(caseScope)
	}

	if s.Init != nil {
		w.finalizeScope(switchScope)
	}
}

// walkTry walks the try body (an ordinary *ast.Block, scoped like any other)
// and, when present, a catch clause. The catch variable and the catch body
// share exactly one scope (matching CLAUDE.md: "catch(e) creates a new local
// variable e scoped to the catch block only"), so the catch *ast.Block's
// statements are walked directly into that scope rather than letting the
// generic *ast.Block case in walkStmt open a second, redundant nested scope.
func (w *walker) walkTry(scope *Scope, s *ast.TryStmt) {
	w.walkStmt(scope, s.Body)

	if s.Catch == nil {
		return
	}

	catchScope := newScope(ScopeBlock, s, scope)
	w.info.registerScope(catchScope)

	if s.CatchVar != "" && s.CatchVar != "_" {
		// TryStmt.CatchVar is a bare string, not an *ast.Ident -- there is no
		// defining occurrence node to record in Info.Defs for it.
		w.info.declare(catchScope, &Symbol{Name: s.CatchVar, Kind: KindVar, DeclNode: s})
	}

	w.walkStmtsIn(catchScope, s.Catch.Stmts)
	w.finalizeScope(catchScope)
}

// declareLabel declares a for-loop label. Labels are function-scoped (like
// Go's), not block-scoped, so it is declared into the nearest FuncBoundary
// rather than the current (possibly nested-block) scope. BreakStmt/
// ContinueStmt targets are bare strings, not *ast.Ident nodes, so label
// "uses" cannot be tracked the same way identifier references are; labels
// are therefore exempt from both UnusedSymbol and UndefinedSymbol/
// UsedBeforeDefined checking (see finalizeScope).
func (w *walker) declareLabel(scope *Scope, s *ast.LabeledStmt) {
	if s.Label == "" {
		return
	}

	target := scope.FuncBoundary
	if target == nil {
		target = scope
	}

	w.info.declare(target, &Symbol{Name: s.Label, Kind: KindLabel, DeclNode: s})
}

// walkFuncDecl walks a function or method declaration -- a top-level one
// (called directly from Resolve) or one nested inside a block (a named,
// non-closure helper function declared inside a bare @test fragment's block,
// e.g. tests/cast/arrays.ego's "func near(a, b float64) bool {...}"; this is
// legal Ego syntax, distinct from the "nested named function cannot access
// enclosing function variable" *compile* restriction CLAUDE.md documents,
// which is about capture, not declaration). It opens the function's own
// ScopeFunction (covering receiver, parameters, and named returns), walks
// the body sequentially, and classifies every function-local symbol's
// register/heap storage suitability once the whole body (including any
// nested function literals) has been walked.
//
// parent is where the name itself is declared. For a top-level FuncDecl,
// collectFileScope (Pass 1) has already declared it into parent (info.Root)
// before any body is walked, precisely so mutually-recursive top-level
// functions resolve regardless of source order -- registerFuncDecl is
// idempotent-checked here (skipped when parent already holds a symbol whose
// DeclNode is this exact node) so that case is not double-registered. A
// nested FuncDecl has no such Pass 1 registration, so this is the only place
// its name is ever declared -- which also means, unlike top-level functions,
// a nested one is only visible to code appearing after it in the same block
// (no forward-reference tolerance), matching ordinary sequential local
// declaration semantics.
func (w *walker) walkFuncDecl(parent *Scope, d *ast.FuncDecl) {
	if d.Name != nil {
		if existing, ok := parent.Symbols[d.Name.Name]; !ok || existing.DeclNode != d {
			registerFuncDecl(w.info, parent, d)
		}
	}

	fnScope := newScope(ScopeFunction, d, parent)
	w.info.registerScope(fnScope)

	if d.Recv != nil {
		w.declareReceiver(fnScope, d.Recv)
	}

	if d.Type != nil {
		w.declareParams(fnScope, d.Type.Params)
		w.declareNamedReturns(fnScope, d.Type.Returns)
	}

	w.pushFuncLocals(d.Body)

	if d.Body != nil {
		w.walkStmtsIn(fnScope, d.Body.Stmts)
	}

	w.popFuncLocals()
	classifyStorageForFunction(fnScope)
	w.finalizeScope(fnScope)
}

// walkFuncLit is walkFuncDecl's counterpart for a function literal
// encountered mid-expression (resolveExpr's *ast.FuncLit case). It is the
// point at which a reference inside the literal's body that resolves to a
// symbol in an enclosing function is detected (see resolveIdent) and
// therefore where closure capture becomes possible at all.
func (w *walker) walkFuncLit(scope *Scope, lit *ast.FuncLit) {
	litScope := newScope(ScopeFunction, lit, scope)
	w.info.registerScope(litScope)

	if lit.Type != nil {
		w.declareParams(litScope, lit.Type.Params)
		w.declareNamedReturns(litScope, lit.Type.Returns)
	}

	w.pushFuncLocals(lit.Body)

	if lit.Body != nil {
		w.walkStmtsIn(litScope, lit.Body.Stmts)
	}

	w.popFuncLocals()
	classifyStorageForFunction(litScope)
	w.finalizeScope(litScope)
}

// declareReceiver declares a method's receiver name.
func (w *walker) declareReceiver(scope *Scope, r *ast.Receiver) {
	if r == nil || r.Name == nil || r.Name.Name == "" || r.Name.Name == "_" {
		return
	}

	w.info.declare(scope, &Symbol{Name: r.Name.Name, Kind: KindReceiver, DeclNode: r, DeclIdent: r.Name, Type: r.Type})
}

// declareParams declares every named parameter across all parameter groups.
func (w *walker) declareParams(scope *Scope, params []*ast.Param) {
	for _, p := range params {
		for _, name := range p.Names {
			if name == nil || name.Name == "" || name.Name == "_" {
				continue
			}

			w.info.declare(scope, &Symbol{Name: name.Name, Kind: KindParam, DeclNode: p, DeclIdent: name, Type: p.Type})
		}
	}
}

// declareNamedReturns declares every named return value.
func (w *walker) declareNamedReturns(scope *Scope, returns []*ast.ReturnItem) {
	for _, r := range returns {
		if r.Name == nil || r.Name.Name == "" || r.Name.Name == "_" {
			continue
		}

		w.info.declare(scope, &Symbol{Name: r.Name.Name, Kind: KindNamedReturn, DeclNode: r, DeclIdent: r.Name, Type: r.Type})
	}
}

// resolveExpr resolves every identifier reference reachable from n against
// scope. Most node kinds need no special handling at all: the default case
// simply recurses into n.Children(), so any new expression node kind added
// to the ast package is handled correctly (its child Idents get resolved)
// without this switch needing to be updated, as long as it does not need one
// of the special exclusion rules below.
//
// Three node kinds are special-cased because resolving every child
// identifier generically would be wrong for them:
//
//   - *ast.Ident itself is the resolution leaf (resolveIdent).
//   - *ast.SelectorExpr's Sel is a member name, never a symbol reference --
//     only X is resolved.
//   - *ast.FuncLit opens a new function scope (walkFuncLit) rather than
//     being walked as an ordinary expression.
//   - *ast.CompositeLit's Type is a syntactic type node, never resolved (see
//     package doc); its Elts need KeyValueExpr-aware handling so a struct
//     field name ("X" in Point{X: 1}) is never mistaken for a variable
//     reference.
func (w *walker) resolveExpr(scope *Scope, n ast.Node) {
	if n == nil {
		return
	}

	switch v := n.(type) {
	case *ast.Ident:
		w.resolveIdent(scope, v)

	case *ast.SelectorExpr:
		w.resolveExpr(scope, v.X)

	case *ast.FuncLit:
		w.walkFuncLit(scope, v)

	case *ast.CompositeLit:
		w.resolveCompositeElts(scope, v)

	default:
		for _, c := range n.Children() {
			w.resolveExpr(scope, c)
		}
	}
}

// resolveCompositeElts walks a composite literal's elements. A struct
// literal's KeyValueExpr.Key is a field name, not a variable reference, and
// is never resolved. A map literal's Key genuinely can be a variable
// reference (map[string]int{myKey: 1}) and is resolved normally. When the
// composite kind is ambiguous (CompositeUnknown, possible when the element
// form could not be determined at parse time), the Key is conservatively
// left unresolved -- preferring under-reporting to a false UndefinedSymbol
// on what may turn out to be a struct field name.
func (w *walker) resolveCompositeElts(scope *Scope, lit *ast.CompositeLit) {
	for _, e := range lit.Elts {
		kv, ok := e.(*ast.KeyValueExpr)
		if !ok {
			w.resolveExpr(scope, e)

			continue
		}

		if lit.Composite == ast.CompositeMap {
			w.resolveExpr(scope, kv.Key)
		}

		w.resolveExpr(scope, kv.Value)
	}
}

// resolveIdent is the resolution leaf: it looks id up in scope's chain,
// records the reference (Info.Uses and the symbol's own Uses list), and
// detects closure capture.
//
// Capture detection: sym.Scope.FuncBoundary is the function the symbol was
// declared in; scope.FuncBoundary is the function the *reference* occurs in
// (both are inherited down through nested block scopes by newScope, so this
// is a single pointer comparison regardless of how many blocks or how many
// nested closures separate the two). They differ exactly when the reference
// is inside a function literal that did not itself declare the symbol --
// i.e. the literal closes over a name from an enclosing function. This is
// evaluated per-symbol, which is strictly finer-grained than the existing
// token-based compiler's whole-function closure disqualification (see
// docs/SLOTS.md Section 5.1); it composes correctly across arbitrarily deep
// nested closures, since a reference inside the innermost literal that
// resolves to an outer ancestor's symbol still fails the equality check
// regardless of how many function boundaries separate them.
func (w *walker) resolveIdent(scope *Scope, id *ast.Ident) {
	if id == nil || id.Name == "" || id.Name == "_" {
		return
	}

	sym := scope.lookup(id.Name)
	if sym == nil {
		w.reportUnresolved(scope, id)

		return
	}

	w.info.Uses[id] = sym
	sym.Uses = append(sym.Uses, id)

	if sym.Scope.FuncBoundary != nil && sym.Scope.FuncBoundary != scope.FuncBoundary {
		sym.Captured = true
		sym.CapturedBy = appendUniqueNode(sym.CapturedBy, scope.FuncBoundary.Node)
	}
}

// appendUniqueNode appends n to list unless it is already present.
func appendUniqueNode(list []ast.Node, n ast.Node) []ast.Node {
	for _, existing := range list {
		if existing == n {
			return list
		}
	}

	return append(list, n)
}

// reportUnresolved records a diagnostic for an identifier that scope.lookup
// could not resolve: UsedBeforeDefined when the name is declared later in
// the same function activation (per currentFuncLocals), UndefinedSymbol
// otherwise -- unless the name is in the builtin allow-list (builtins.go),
// in which case no diagnostic is produced at all.
func (w *walker) reportUnresolved(scope *Scope, id *ast.Ident) {
	_ = scope

	if isPredefined(id.Name) {
		return
	}

	if locals := w.currentFuncLocals(); locals != nil {
		if pos, ok := locals[id.Name]; ok {
			w.diags = append(w.diags, Diagnostic{
				Pos:     id.Pos(),
				Kind:    UsedBeforeDefined,
				Name:    id.Name,
				Message: fmt.Sprintf("%s used before it is defined (declared later, at line %d)", id.Name, pos.Line),
			})

			return
		}
	}

	w.diags = append(w.diags, Diagnostic{
		Pos:     id.Pos(),
		Kind:    UndefinedSymbol,
		Name:    id.Name,
		Message: "undefined: " + id.Name,
	})
}

// finalizeScope emits an UnusedSymbol diagnostic for every symbol declared
// directly in scope that was never read. This only ever applies to local
// variables and constants (KindVar, KindConst): matching Go's own "declared
// and not used" rule, an unused parameter, named return, receiver, function,
// type, import, or label is never flagged. File-scope symbols are also
// exempt entirely -- see the package doc's single-file limitation.
func (w *walker) finalizeScope(scope *Scope) {
	if scope.Kind == ScopeFile {
		return
	}

	for _, sym := range scope.Order {
		if len(sym.Uses) > 0 {
			continue
		}

		if sym.Kind != KindVar && sym.Kind != KindConst {
			continue
		}

		if sym.Name == "_" {
			continue
		}

		w.diags = append(w.diags, Diagnostic{
			Pos:     sym.Pos(),
			Kind:    UnusedSymbol,
			Name:    sym.Name,
			Message: sym.Name + " declared and not used",
		})
	}
}

// classifyStorageForFunction sets Storage on every Var/Const/Param/
// NamedReturn/Receiver symbol declared anywhere within fnScope's own subtree
// -- StorageHeap if Captured, StorageRegister otherwise -- once fnScope's
// entire body has been walked (so every capture that will ever be detected
// already has been). It stops at nested ScopeFunction children (nested
// function literals), which are classified independently, at the point
// walkFuncLit finishes walking each one.
func classifyStorageForFunction(fnScope *Scope) {
	for _, sym := range fnScope.Order {
		switch sym.Kind {
		case KindVar, KindConst, KindParam, KindNamedReturn, KindReceiver:
			if sym.Captured {
				sym.Storage = StorageHeap
			} else {
				sym.Storage = StorageRegister
			}
		}
	}

	for _, child := range fnScope.Children {
		if child.Kind == ScopeFunction {
			continue
		}

		classifyStorageForFunction(child)
	}
}
