package ast

// This file defines statement node types, corresponding to docs/SYNTAX.md
// sections 3, 8, 10, and 11.

// Block is "{ Stmts }", a brace-delimited statement sequence that opens a new
// scope. Empty is true for the "{}" empty-block shorthand.
type Block struct {
	BaseNode
	Stmts []Node
	Empty bool
}

func (n *Block) Kind() Kind       { return KindBlock }
func (n *Block) Children() []Node { return nodes(n.Stmts) }
func (n *Block) String() string   { return "Block" }

// EmptyStmt is a bare ";" or "{}" no-op statement.
type EmptyStmt struct {
	BaseNode
}

func (n *EmptyStmt) Kind() Kind       { return KindEmptyStmt }
func (n *EmptyStmt) Children() []Node { return nil }
func (n *EmptyStmt) String() string   { return "EmptyStmt" }

// ExprStmt is an expression used as a statement (typically a function call).
type ExprStmt struct {
	BaseNode
	X Node
}

func (n *ExprStmt) Kind() Kind       { return KindExprStmt }
func (n *ExprStmt) Children() []Node { return nodes(n.X) }
func (n *ExprStmt) String() string   { return "ExprStmt" }

// AssignStmt is an assignment "Lhs Op Rhs". Op is the assignment operator
// spelling: "=", ":=", "+=", "-=", "*=", or "/=".
type AssignStmt struct {
	BaseNode
	Lhs []Node
	Op  string
	Rhs []Node
}

func (n *AssignStmt) Kind() Kind       { return KindAssignStmt }
func (n *AssignStmt) Children() []Node { return nodes(n.Lhs, n.Rhs) }
func (n *AssignStmt) String() string   { return "AssignStmt(" + n.Op + ")" }

// IncDecStmt is "X++" or "X--". Op is "++" or "--".
type IncDecStmt struct {
	BaseNode
	X  Node
	Op string
}

func (n *IncDecStmt) Kind() Kind       { return KindIncDecStmt }
func (n *IncDecStmt) Children() []Node { return nodes(n.X) }
func (n *IncDecStmt) String() string   { return "IncDecStmt(" + n.Op + ")" }

// SendStmt is a channel send "Chan <- Value".
type SendStmt struct {
	BaseNode
	Chan  Node
	Value Node
}

func (n *SendStmt) Kind() Kind       { return KindSendStmt }
func (n *SendStmt) Children() []Node { return nodes(n.Chan, n.Value) }
func (n *SendStmt) String() string   { return "SendStmt" }

// ReturnStmt is "return [Results]".
type ReturnStmt struct {
	BaseNode
	Results []Node
}

func (n *ReturnStmt) Kind() Kind       { return KindReturnStmt }
func (n *ReturnStmt) Children() []Node { return nodes(n.Results) }
func (n *ReturnStmt) String() string   { return "ReturnStmt" }

// BreakStmt is "break [Label]".
type BreakStmt struct {
	BaseNode
	Label string
}

func (n *BreakStmt) Kind() Kind       { return KindBreakStmt }
func (n *BreakStmt) Children() []Node { return nil }
func (n *BreakStmt) String() string   { return "BreakStmt" }

// ContinueStmt is "continue [Label]".
type ContinueStmt struct {
	BaseNode
	Label string
}

func (n *ContinueStmt) Kind() Kind       { return KindContinueStmt }
func (n *ContinueStmt) Children() []Node { return nil }
func (n *ContinueStmt) String() string   { return "ContinueStmt" }

// LabeledStmt is "Label: Stmt". In Ego the only labeled statement is a for
// loop (labels target loops for break/continue).
type LabeledStmt struct {
	BaseNode
	Label string
	Stmt  Node
}

func (n *LabeledStmt) Kind() Kind       { return KindLabeledStmt }
func (n *LabeledStmt) Children() []Node { return nodes(n.Stmt) }
func (n *LabeledStmt) String() string   { return "LabeledStmt(" + n.Label + ")" }

// DeferStmt is "defer Call".
type DeferStmt struct {
	BaseNode
	Call Node
}

func (n *DeferStmt) Kind() Kind       { return KindDeferStmt }
func (n *DeferStmt) Children() []Node { return nodes(n.Call) }
func (n *DeferStmt) String() string   { return "DeferStmt" }

// GoStmt is "go Call".
type GoStmt struct {
	BaseNode
	Call Node
}

func (n *GoStmt) Kind() Kind       { return KindGoStmt }
func (n *GoStmt) Children() []Node { return nodes(n.Call) }
func (n *GoStmt) String() string   { return "GoStmt" }

// IfStmt is "if [Init;] Cond Body [else Else]". Init may be nil; Else is either
// a *Block or another *IfStmt (the else-if chain), or nil.
type IfStmt struct {
	BaseNode
	Init Node
	Cond Node
	Body *Block
	Else Node
}

func (n *IfStmt) Kind() Kind       { return KindIfStmt }
func (n *IfStmt) Children() []Node { return nodes(n.Init, n.Cond, n.Body, n.Else) }
func (n *IfStmt) String() string   { return "IfStmt" }

// ForStmt covers all four for-loop forms. The populated fields select the form:
//
//   - infinite loop:    all of Init/Cond/Post/Key/Value/Range nil
//   - conditional loop: only Cond set
//   - three-clause:     Init, Cond, Post set
//   - range loop:       Key (and optionally Value) and Range set
//
// Define is true when a range loop introduced its variables with ":=".
type ForStmt struct {
	BaseNode
	Init   Node
	Cond   Node
	Post   Node
	Key    Node
	Value  Node
	Range  Node
	Define bool
	Body   *Block
}

func (n *ForStmt) Kind() Kind { return KindForStmt }

func (n *ForStmt) Children() []Node {
	return nodes(n.Init, n.Cond, n.Post, n.Key, n.Value, n.Range, n.Body)
}

func (n *ForStmt) String() string { return "ForStmt" }

// SwitchStmt is "switch [Init;] [Tag] { Body }". Init and Tag may be nil (a
// bare "switch { }" evaluates each case as a boolean condition).
type SwitchStmt struct {
	BaseNode
	Init Node
	Tag  Node
	Body []*CaseClause
}

func (n *SwitchStmt) Kind() Kind { return KindSwitchStmt }

func (n *SwitchStmt) Children() []Node {
	var children []Node

	children = append(children, nodes(n.Init, n.Tag)...)

	for _, c := range n.Body {
		children = append(children, c)
	}

	return children
}

func (n *SwitchStmt) String() string { return "SwitchStmt" }

// CaseClause is one "case Exprs: Body" or "default: Body" clause. Default is
// true for the default clause (Exprs is then empty). Fallthrough is true when
// the body ends with a "fallthrough" statement.
type CaseClause struct {
	BaseNode
	Default     bool
	Exprs       []Node
	Body        []Node
	Fallthrough bool
}

func (n *CaseClause) Kind() Kind       { return KindCaseClause }
func (n *CaseClause) Children() []Node { return nodes(n.Exprs, n.Body) }
func (n *CaseClause) String() string   { return "CaseClause" }

// TryStmt is the extension "try Body [catch [(Ident)] Catch]". CatchVar is the
// bound error variable name ("" when omitted); Catch may be nil.
type TryStmt struct {
	BaseNode
	Body     *Block
	CatchVar string
	Catch    *Block
}

func (n *TryStmt) Kind() Kind       { return KindTryStmt }
func (n *TryStmt) Children() []Node { return nodes(n.Body, n.Catch) }
func (n *TryStmt) String() string   { return "TryStmt" }

// PanicStmt is the extension "panic(Arg)". Arg may be nil.
type PanicStmt struct {
	BaseNode
	Arg Node
}

func (n *PanicStmt) Kind() Kind       { return KindPanicStmt }
func (n *PanicStmt) Children() []Node { return nodes(n.Arg) }
func (n *PanicStmt) String() string   { return "PanicStmt" }

// PrintStmt is the extension "print [Args] [,]". NoNewline is true when a
// trailing comma suppressed the automatic newline.
type PrintStmt struct {
	BaseNode
	Args      []Node
	NoNewline bool
}

func (n *PrintStmt) Kind() Kind       { return KindPrintStmt }
func (n *PrintStmt) Children() []Node { return nodes(n.Args) }
func (n *PrintStmt) String() string   { return "PrintStmt" }

// CallStmt is the extension "call Call".
type CallStmt struct {
	BaseNode
	Call Node
}

func (n *CallStmt) Kind() Kind       { return KindCallStmt }
func (n *CallStmt) Children() []Node { return nodes(n.Call) }
func (n *CallStmt) String() string   { return "CallStmt" }

// ThrowStmt is the extension "throw Expr".
type ThrowStmt struct {
	BaseNode
	X Node
}

func (n *ThrowStmt) Kind() Kind       { return KindThrowStmt }
func (n *ThrowStmt) Children() []Node { return nodes(n.X) }
func (n *ThrowStmt) String() string   { return "ThrowStmt" }

// ExitStmt is the REPL-only "exit [Code]". Code may be nil.
type ExitStmt struct {
	BaseNode
	Code Node
}

func (n *ExitStmt) Kind() Kind       { return KindExitStmt }
func (n *ExitStmt) Children() []Node { return nodes(n.Code) }
func (n *ExitStmt) String() string   { return "ExitStmt" }

// DirectiveStmt is a compile-time directive "@Name Args". Because Ego has ~40
// directives (and treats unknown ones as macro invocations), the AST models
// them generically: Name is the directive identifier and Args holds any parsed
// argument expressions. RawArgs preserves the trailing token spellings for
// directives whose arguments are not ordinary expressions (e.g. @endpoint,
// @compile), so a consumer can re-read them without the parser needing bespoke
// structure for every directive.
type DirectiveStmt struct {
	BaseNode
	Name    string
	Args    []Node
	RawArgs []string
}

func (n *DirectiveStmt) Kind() Kind       { return KindDirectiveStmt }
func (n *DirectiveStmt) Children() []Node { return nodes(n.Args) }
func (n *DirectiveStmt) String() string   { return "DirectiveStmt(@" + n.Name + ")" }
