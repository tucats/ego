package ast

// This file defines the SELECT statement and the clause nodes it shares with
// other DML statements (table references, joins, ORDER BY terms, LIMIT).

// SelectStmt is a complete SELECT statement: an optional WITH clause, one or
// more SELECT cores combined with compound operators, and a trailing
// ORDER BY / LIMIT that apply to the statement as a whole. Select is either a
// *SelectCore (a single, non-compound SELECT) or a *CompoundSelect (two or
// more cores joined by UNION/INTERSECT/EXCEPT).
type SelectStmt struct {
	BaseStmt
	With    *WithClause
	Select  Node
	OrderBy []*OrderByTerm
	Limit   *LimitClause
}

func (n *SelectStmt) Kind() Kind { return KindSelectStmt }
func (n *SelectStmt) Children() []Node {
	// See the comment on CaseExpr.Children in expr.go for why this manual
	// []*OrderByTerm -> []Node copy loop is needed before handing the slice
	// to nodes().
	orderBy := make([]Node, len(n.OrderBy))
	for i, o := range n.OrderBy {
		orderBy[i] = o
	}

	return nodes(n.With, n.Select, orderBy, n.Limit)
}
func (n *SelectStmt) String() string { return "SelectStmt" }

// CompoundSelect is two SELECT cores joined by a compound operator: Left Op
// Right. Op is one of "UNION", "UNION ALL", "INTERSECT", or "EXCEPT". Left
// nesting (rather than a flat slice) preserves left-associativity for
// statements with more than two cores. Left is a *SelectCore or, for a
// chain of three or more, a nested *CompoundSelect.
type CompoundSelect struct {
	BaseNode
	Left  Node
	Op    string
	Right *SelectCore
}

func (n *CompoundSelect) Kind() Kind       { return KindCompoundSelect }
func (n *CompoundSelect) Children() []Node { return nodes(n.Left, n.Right) }
func (n *CompoundSelect) String() string   { return "CompoundSelect(" + n.Op + ")" }

// SelectCore is a single "SELECT ... FROM ... WHERE ..." without any
// compound operator, ORDER BY, or LIMIT (those belong to the enclosing
// SelectStmt).
type SelectCore struct {
	BaseNode
	Distinct bool
	All      bool
	Columns  []*ResultColumn
	From     []Node
	Where    Node
	GroupBy  []Node
	Having   Node
}

func (n *SelectCore) Kind() Kind { return KindSelectCore }
func (n *SelectCore) Children() []Node {
	cols := make([]Node, len(n.Columns))
	for i, c := range n.Columns {
		cols[i] = c
	}

	return nodes(cols, n.From, n.Where, n.GroupBy, n.Having)
}
func (n *SelectCore) String() string { return "SelectCore" }

// WithClause is a "WITH [RECURSIVE] name AS (select), ..." prefix attached
// to a SelectStmt (or, in dialects that allow it, an Insert/Update/Delete —
// represented the same way on those statements).
type WithClause struct {
	BaseNode
	Recursive bool
	CTEs      []*CTE
}

func (n *WithClause) Kind() Kind { return KindWithClause }
func (n *WithClause) Children() []Node {
	ctes := make([]Node, len(n.CTEs))
	for i, c := range n.CTEs {
		ctes[i] = c
	}

	return nodes(ctes)
}
func (n *WithClause) String() string { return "WithClause" }

// CTE is one "name [(columns...)] AS (Select)" entry of a WithClause. Select
// is a *SelectStmt or *CompoundSelect.
type CTE struct {
	BaseNode
	Name    string
	Columns []string
	Select  Node
}

func (n *CTE) Kind() Kind       { return KindCTE }
func (n *CTE) Children() []Node { return nodes(n.Select) }
func (n *CTE) String() string   { return "CTE(" + n.Name + ")" }

// ResultColumn is one entry of a SELECT list: an expression with an optional
// alias, or "*" / "table.*" (in which case Expr is a *StarExpr and Alias is
// empty — aliasing a star is not legal SQL).
type ResultColumn struct {
	BaseNode
	Expr  Node
	Alias string
}

func (n *ResultColumn) Kind() Kind       { return KindResultColumn }
func (n *ResultColumn) Children() []Node { return nodes(n.Expr) }
func (n *ResultColumn) String() string {
	if n.Alias == "" {
		return "ResultColumn"
	}

	return "ResultColumn(AS " + n.Alias + ")"
}

// TableRef is a table reference in a FROM (or UPDATE/DELETE/INSERT INTO)
// clause: [Schema.]Name [[AS] Alias]. IndexedBy and NotIndexed capture
// sqlite3's optional index hint; both are zero for ordinary references and
// for PostgreSQL, which has no equivalent syntax.
type TableRef struct {
	BaseNode
	Schema     string
	Name       string
	Alias      string
	IndexedBy  string
	NotIndexed bool
}

func (n *TableRef) Kind() Kind       { return KindTableRef }
func (n *TableRef) Children() []Node { return nil }
func (n *TableRef) String() string {
	if n.Schema == "" {
		return "TableRef(" + n.Name + ")"
	}

	return "TableRef(" + n.Schema + "." + n.Name + ")"
}

// SubqueryRef is a parenthesized SELECT used as a FROM-clause item:
// (Select) [AS] Alias [(Columns...)].
type SubqueryRef struct {
	BaseNode
	Sub     *Subquery
	Alias   string
	Columns []string
}

func (n *SubqueryRef) Kind() Kind       { return KindSubqueryRef }
func (n *SubqueryRef) Children() []Node { return nodes(n.Sub) }
func (n *SubqueryRef) String() string   { return "SubqueryRef(AS " + n.Alias + ")" }

// JoinClause is "Left JoinType JOIN Right [ON On | USING (Using...)]".
// JoinType is a normalized, upper-case spelling of the join keywords that
// preceded JOIN: "", "INNER", "LEFT", "LEFT OUTER", "RIGHT", "RIGHT OUTER",
// "FULL", "FULL OUTER", "CROSS", "NATURAL", "NATURAL LEFT", etc. An empty
// On with a nil Using and a non-NATURAL, non-CROSS JoinType represents a
// join with neither ON nor USING, which sqlite3 permits (it degenerates to a
// cross join).
type JoinClause struct {
	BaseNode
	Left     Node
	JoinType string
	Right    Node
	On       Node
	Using    []string
}

func (n *JoinClause) Kind() Kind       { return KindJoinClause }
func (n *JoinClause) Children() []Node { return nodes(n.Left, n.Right, n.On) }
func (n *JoinClause) String() string   { return "JoinClause(" + n.JoinType + ")" }

// OrderByTerm is one "Expr [COLLATE Collation] [ASC|DESC] [NULLS FIRST|LAST]"
// entry of an ORDER BY clause. NullsFirst is a *bool rather than a bool so it
// can represent three states instead of two: nil means "NULLS FIRST/LAST was
// not written at all" (the dialect's default applies), a pointer to true
// means "NULLS FIRST", and a pointer to false means "NULLS LAST". A plain
// bool can't distinguish "not specified" from "specified as false", since
// both would just be the zero value false — this is the standard Go pattern
// for an optional/tri-state flag. Code that reads this field checks for nil
// first (e.g. "if term.NullsFirst != nil && *term.NullsFirst { ... }").
type OrderByTerm struct {
	BaseNode
	Expr       Node
	Collation  string
	Desc       bool
	NullsFirst *bool
}

func (n *OrderByTerm) Kind() Kind       { return KindOrderByTerm }
func (n *OrderByTerm) Children() []Node { return nodes(n.Expr) }
func (n *OrderByTerm) String() string   { return "OrderByTerm" }

// LimitClause is "LIMIT Limit [OFFSET Offset]". Offset is nil when omitted.
type LimitClause struct {
	BaseNode
	Limit  Node
	Offset Node
}

func (n *LimitClause) Kind() Kind       { return KindLimitClause }
func (n *LimitClause) Children() []Node { return nodes(n.Limit, n.Offset) }
func (n *LimitClause) String() string   { return "LimitClause" }
