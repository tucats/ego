package ast

// This file defines expression node types: literals, references, operators,
// function calls, and subquery expressions. None of these implement
// Statement — they only ever appear as children of a statement or clause
// node (e.g. inside a WHERE, ON, or SET clause).
//
// Every node type in this file (and in stmt.go, ddl.go, dml.go, and txn.go)
// follows the same shape, so it's worth understanding it once here rather
// than re-explaining it at each of the ~50 repetitions:
//
//	type SomeNode struct {
//		BaseNode          // embedded — gives SomeNode working Pos()/End()/SetSpan() for free
//		FieldA string      // the node's own data
//		FieldB Node
//	}
//
//	func (n *SomeNode) Kind() Kind       { return KindSomeNode }
//	func (n *SomeNode) Children() []Node { return nodes(n.FieldB) }
//	func (n *SomeNode) String() string   { return "SomeNode(...)" }
//
// The three explicit methods, plus the three promoted from the embedded
// BaseNode, are exactly the six methods the Node interface in node.go
// requires — that's the whole reason every node type is shaped this way.
// Kind() just returns a fixed constant per type (see kind.go); Children()
// reports the node's direct child nodes via the nodes() helper in visit.go,
// which is what lets Walk traverse an arbitrary tree without knowing about
// any specific node type; and String() is a short debug label, not source
// reconstruction. Note the methods are defined on the pointer type (*SomeNode,
// not SomeNode) — parser code always builds these with &ast.SomeNode{...} and
// hands the pointer around, so a *SomeNode is what actually flows through
// the tree as a Node interface value.
//
// ColumnRef is a (possibly qualified) column reference: Column, Table.Column,
// or Schema.Table.Column. Schema and Table are empty when not written.
type ColumnRef struct {
	BaseNode
	Schema string
	Table  string
	Column string
}

func (n *ColumnRef) Kind() Kind       { return KindColumnRef }
func (n *ColumnRef) Children() []Node { return nil }
func (n *ColumnRef) String() string   { return "ColumnRef(" + n.qualified() + ")" }

func (n *ColumnRef) qualified() string {
	s := n.Column
	if n.Table != "" {
		s = n.Table + "." + s
	}

	if n.Schema != "" {
		s = n.Schema + "." + s
	}

	return s
}

// StarExpr is "*" or "table.*" used as a SELECT result column or as the sole
// argument of an aggregate such as COUNT(*) (in the latter case it appears as
// FuncCall.Star instead; StarExpr is only used in result-column position).
type StarExpr struct {
	BaseNode
	Table string
}

func (n *StarExpr) Kind() Kind       { return KindStarExpr }
func (n *StarExpr) Children() []Node { return nil }
func (n *StarExpr) String() string {
	if n.Table == "" {
		return "StarExpr(*)"
	}

	return "StarExpr(" + n.Table + ".*)"
}

// LitKind categorizes a Literal's value form.
type LitKind int

const (
	// LitInvalid is the zero value.
	LitInvalid LitKind = iota
	LitInteger
	LitFloat
	LitString
	LitBlob
	LitBool
	LitNull
)

var litKindNames = map[LitKind]string{
	LitInvalid: "invalid",
	LitInteger: "integer",
	LitFloat:   "float",
	LitString:  "string",
	LitBlob:    "blob",
	LitBool:    "bool",
	LitNull:    "null",
}

// String returns the name of the literal kind.
func (k LitKind) String() string {
	if name, ok := litKindNames[k]; ok {
		return name
	}

	return "LitKind(" + itoa(int(k)) + ")"
}

// Literal is a literal scalar value: integer, float, string, blob, boolean,
// or NULL. Value holds the source spelling with quoting/prefix already
// stripped (e.g. a string literal's surrounding quotes are removed and any
// doubled quote escape is collapsed to one; a blob literal's leading "X" and
// surrounding quotes are removed, leaving just the hex digits).
type Literal struct {
	BaseNode
	LitKind LitKind
	Value   string
}

func (n *Literal) Kind() Kind       { return KindLiteral }
func (n *Literal) Children() []Node { return nil }
func (n *Literal) String() string   { return "Literal(" + n.LitKind.String() + ":" + n.Value + ")" }

// PlaceholderStyle distinguishes the spellings of bind parameter accepted by
// sqlite3 and PostgreSQL.
type PlaceholderStyle int

const (
	// PlaceholderAnonymous is sqlite3's "?" form.
	PlaceholderAnonymous PlaceholderStyle = iota

	// PlaceholderNumbered is sqlite3's "?NNN" form or PostgreSQL's "$NNN" form.
	PlaceholderNumbered

	// PlaceholderNamed is sqlite3's ":name", "@name", or "$name" form.
	PlaceholderNamed
)

// Placeholder is a bind parameter. Text holds the full source spelling
// (including any leading "?", "$", ":", or "@" marker) so that a formatter
// can reproduce it exactly.
type Placeholder struct {
	BaseNode
	Style PlaceholderStyle
	Text  string
}

func (n *Placeholder) Kind() Kind       { return KindPlaceholder }
func (n *Placeholder) Children() []Node { return nil }
func (n *Placeholder) String() string   { return "Placeholder(" + n.Text + ")" }

// UnaryExpr is a prefix unary operation: Op X, where Op is one of "-", "+",
// "~", or "NOT".
type UnaryExpr struct {
	BaseNode
	Op string
	X  Node
}

func (n *UnaryExpr) Kind() Kind       { return KindUnaryExpr }
func (n *UnaryExpr) Children() []Node { return nodes(n.X) }
func (n *UnaryExpr) String() string   { return "UnaryExpr(" + n.Op + ")" }

// BinaryExpr is a binary operation: X Op Y. Op holds the operator spelling,
// normalized to upper case for word operators (e.g. "AND", "OR") and left
// as-is for symbolic operators (e.g. "+", "||", "->>").
type BinaryExpr struct {
	BaseNode
	Op string
	X  Node
	Y  Node
}

func (n *BinaryExpr) Kind() Kind       { return KindBinaryExpr }
func (n *BinaryExpr) Children() []Node { return nodes(n.X, n.Y) }
func (n *BinaryExpr) String() string   { return "BinaryExpr(" + n.Op + ")" }

// BetweenExpr is "X [NOT] BETWEEN Low AND High".
type BetweenExpr struct {
	BaseNode
	X    Node
	Not  bool
	Low  Node
	High Node
}

func (n *BetweenExpr) Kind() Kind       { return KindBetweenExpr }
func (n *BetweenExpr) Children() []Node { return nodes(n.X, n.Low, n.High) }
func (n *BetweenExpr) String() string {
	if n.Not {
		return "BetweenExpr(NOT)"
	}

	return "BetweenExpr"
}

// InExpr is "X [NOT] IN (List...)" or "X [NOT] IN (Sub)". Exactly one of
// List or Sub is set.
type InExpr struct {
	BaseNode
	X    Node
	Not  bool
	List []Node
	Sub  *Subquery
}

func (n *InExpr) Kind() Kind       { return KindInExpr }
func (n *InExpr) Children() []Node { return nodes(n.X, n.List, n.Sub) }
func (n *InExpr) String() string {
	if n.Not {
		return "InExpr(NOT)"
	}

	return "InExpr"
}

// LikeExpr is "X [NOT] Op Pattern [ESCAPE Escape]", where Op is one of
// "LIKE", "ILIKE" (PostgreSQL), "GLOB", "REGEXP", or "MATCH" (sqlite3).
// Escape is nil unless an ESCAPE clause was written.
type LikeExpr struct {
	BaseNode
	X       Node
	Not     bool
	Op      string
	Pattern Node
	Escape  Node
}

func (n *LikeExpr) Kind() Kind       { return KindLikeExpr }
func (n *LikeExpr) Children() []Node { return nodes(n.X, n.Pattern, n.Escape) }
func (n *LikeExpr) String() string   { return "LikeExpr(" + n.Op + ")" }

// IsNullExpr is "X IS NULL" or "X IS NOT NULL" (the latter also written as
// "X NOTNULL" or "X ISNULL" in sqlite3, which the parser normalizes to this
// same node).
type IsNullExpr struct {
	BaseNode
	X   Node
	Not bool
}

func (n *IsNullExpr) Kind() Kind       { return KindIsNullExpr }
func (n *IsNullExpr) Children() []Node { return nodes(n.X) }
func (n *IsNullExpr) String() string {
	if n.Not {
		return "IsNullExpr(NOT)"
	}

	return "IsNullExpr"
}

// IsExpr is "X IS [NOT] [DISTINCT FROM] Y", covering both the boolean-test
// form (Y a TRUE/FALSE/UNKNOWN literal) and the DISTINCT FROM comparison
// form.
type IsExpr struct {
	BaseNode
	X        Node
	Not      bool
	Distinct bool
	Y        Node
}

func (n *IsExpr) Kind() Kind       { return KindIsExpr }
func (n *IsExpr) Children() []Node { return nodes(n.X, n.Y) }
func (n *IsExpr) String() string   { return "IsExpr" }

// CollateExpr is "X COLLATE Collation".
type CollateExpr struct {
	BaseNode
	X         Node
	Collation string
}

func (n *CollateExpr) Kind() Kind       { return KindCollateExpr }
func (n *CollateExpr) Children() []Node { return nodes(n.X) }
func (n *CollateExpr) String() string   { return "CollateExpr(" + n.Collation + ")" }

// FuncCall is a function call: Name(Args...). Distinct is true for
// "Name(DISTINCT Args...)". Star is true for the special "Name(*)" form
// (e.g. COUNT(*)), in which case Args is empty. Filter holds the expression
// of a trailing "FILTER (WHERE ...)" clause when present, else nil. The
// parser does not validate the function name or argument count/types —
// functions may be dialect extensions.
type FuncCall struct {
	BaseNode
	Name     string
	Distinct bool
	Star     bool
	Args     []Node
	Filter   Node
}

func (n *FuncCall) Kind() Kind       { return KindFuncCall }
func (n *FuncCall) Children() []Node { return nodes(n.Args, n.Filter) }
func (n *FuncCall) String() string   { return "FuncCall(" + n.Name + ")" }

// CastExpr is "CAST(X AS Type)".
type CastExpr struct {
	BaseNode
	X    Node
	Type *TypeName
}

func (n *CastExpr) Kind() Kind       { return KindCastExpr }
func (n *CastExpr) Children() []Node { return nodes(n.X, n.Type) }
func (n *CastExpr) String() string   { return "CastExpr" }

// WhenClause is one "WHEN Cond THEN Result" arm of a CaseExpr.
type WhenClause struct {
	BaseNode
	Cond   Node
	Result Node
}

func (n *WhenClause) Kind() Kind       { return KindWhenClause }
func (n *WhenClause) Children() []Node { return nodes(n.Cond, n.Result) }
func (n *WhenClause) String() string   { return "WhenClause" }

// CaseExpr is a CASE expression. Operand is set for the simple form
// "CASE Operand WHEN ... END" and nil for the searched form
// "CASE WHEN Cond ... END". Else is nil when no ELSE clause was written.
type CaseExpr struct {
	BaseNode
	Operand Node
	Whens   []*WhenClause
	Else    Node
}

func (n *CaseExpr) Kind() Kind { return KindCaseExpr }
func (n *CaseExpr) Children() []Node {
	// Whens is []*WhenClause, not []Node, even though *WhenClause satisfies
	// Node — Go does not treat []*WhenClause and []Node as interchangeable
	// (a []T is a distinct type from []I even when T implements I), so
	// nodes() can't accept n.Whens directly. This manual copy converts each
	// element individually. The same pattern shows up anywhere a field is a
	// slice of a concrete node type rather than []Node (see e.g.
	// SelectStmt.Children in stmt.go for another example).
	whens := make([]Node, len(n.Whens))
	for i, w := range n.Whens {
		whens[i] = w
	}

	return nodes(n.Operand, whens, n.Else)
}
func (n *CaseExpr) String() string { return "CaseExpr" }

// ParenExpr is a parenthesized expression: "(" X ")". It is retained (rather
// than discarded during parsing) so that a formatter can reproduce the
// source grouping.
type ParenExpr struct {
	BaseNode
	X Node
}

func (n *ParenExpr) Kind() Kind       { return KindParenExpr }
func (n *ParenExpr) Children() []Node { return nodes(n.X) }
func (n *ParenExpr) String() string   { return "ParenExpr" }

// ExistsExpr is "[NOT] EXISTS (Sub)".
type ExistsExpr struct {
	BaseNode
	Not bool
	Sub *Subquery
}

func (n *ExistsExpr) Kind() Kind       { return KindExistsExpr }
func (n *ExistsExpr) Children() []Node { return nodes(n.Sub) }
func (n *ExistsExpr) String() string {
	if n.Not {
		return "ExistsExpr(NOT)"
	}

	return "ExistsExpr"
}

// Subquery is a parenthesized SELECT used in expression position: as a
// scalar subquery, or as the operand of IN/EXISTS. Select is either a
// *SelectStmt or a *CompoundSelect.
type Subquery struct {
	BaseNode
	Select Node
}

func (n *Subquery) Kind() Kind       { return KindSubquery }
func (n *Subquery) Children() []Node { return nodes(n.Select) }
func (n *Subquery) String() string   { return "Subquery" }

// ExprList is a parenthesized, comma-separated list of expressions used
// where the grammar calls for a tuple rather than a single scalar — for
// example the left-hand side of a row-value IN comparison: "(a, b) IN (...)".
type ExprList struct {
	BaseNode
	Items []Node
}

func (n *ExprList) Kind() Kind       { return KindExprList }
func (n *ExprList) Children() []Node { return nodes(n.Items) }
func (n *ExprList) String() string   { return "ExprList" }
