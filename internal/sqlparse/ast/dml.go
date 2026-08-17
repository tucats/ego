package ast

// This file defines INSERT, UPDATE, and DELETE and the clauses specific to
// them (SET, ON CONFLICT / DO UPDATE, RETURNING).

// InsertStmt is "INSERT [OrAction] INTO Table [(Columns...)] Source
// [OnConflict] [Returning]". OrAction holds sqlite3's optional conflict
// resolution keyword ("REPLACE", "IGNORE", "ABORT", "FAIL", "ROLLBACK"),
// empty when not written. Source is one of *InsertValues, *InsertSelect, or
// *InsertDefaultValues.
type InsertStmt struct {
	BaseStmt
	With       *WithClause
	OrAction   string
	Table      *TableRef
	Columns    []string
	Source     Node
	OnConflict *OnConflictClause
	Returning  *ReturningClause
}

func (n *InsertStmt) Kind() Kind { return KindInsertStmt }
func (n *InsertStmt) Children() []Node {
	return nodes(n.With, n.Table, n.Source, n.OnConflict, n.Returning)
}
func (n *InsertStmt) String() string { return "InsertStmt" }

// InsertValues is the "VALUES (Rows[0]...), (Rows[1]...), ..." form of an
// INSERT source.
type InsertValues struct {
	BaseNode
	Rows [][]Node
}

func (n *InsertValues) Kind() Kind { return KindInsertValues }
func (n *InsertValues) Children() []Node {
	// Rows is [][]Node — one []Node per parenthesized VALUES row. This
	// flattens it into a single []Node. "row..." here expands the row slice
	// into individual arguments for append, the same "..." used to call a
	// variadic function with an existing slice instead of listing elements
	// one by one (append itself is variadic: func append(s []T, vs ...T) []T).
	var all []Node
	for _, row := range n.Rows {
		all = append(all, row...)
	}

	return nodes(all)
}
func (n *InsertValues) String() string { return "InsertValues" }

// InsertSelect is the "SELECT ..." form of an INSERT source. Select is a
// *SelectStmt or *CompoundSelect.
type InsertSelect struct {
	BaseNode
	Select Node
}

func (n *InsertSelect) Kind() Kind       { return KindInsertSelect }
func (n *InsertSelect) Children() []Node { return nodes(n.Select) }
func (n *InsertSelect) String() string   { return "InsertSelect" }

// InsertDefaultValues is the "DEFAULT VALUES" form of an INSERT source.
type InsertDefaultValues struct {
	BaseNode
}

func (n *InsertDefaultValues) Kind() Kind       { return KindInsertDefaultValues }
func (n *InsertDefaultValues) Children() []Node { return nil }
func (n *InsertDefaultValues) String() string   { return "InsertDefaultValues" }

// UpdateStmt is "UPDATE [OrAction] Table SET Set... [FROM From...]
// [WHERE Where] [Returning]". From is only legal in PostgreSQL; it is empty
// for sqlite3 source.
type UpdateStmt struct {
	BaseStmt
	With      *WithClause
	OrAction  string
	Table     *TableRef
	Set       []*SetClause
	From      []Node
	Where     Node
	Returning *ReturningClause
}

func (n *UpdateStmt) Kind() Kind { return KindUpdateStmt }
func (n *UpdateStmt) Children() []Node {
	set := make([]Node, len(n.Set))
	for i, s := range n.Set {
		set[i] = s
	}

	return nodes(n.With, n.Table, set, n.From, n.Where, n.Returning)
}
func (n *UpdateStmt) String() string { return "UpdateStmt" }

// SetClause is one assignment of an UPDATE's SET list. Columns holds a
// single name for the ordinary "col = expr" form, or two or more names for
// the row-value form "(col1, col2) = (expr1, expr2)".
type SetClause struct {
	BaseNode
	Columns []string
	Value   Node
}

func (n *SetClause) Kind() Kind       { return KindSetClause }
func (n *SetClause) Children() []Node { return nodes(n.Value) }
func (n *SetClause) String() string   { return "SetClause" }

// DeleteStmt is "DELETE FROM Table [USING Using...] [WHERE Where]
// [Returning]". Using is only legal in PostgreSQL; it is empty for sqlite3
// source.
type DeleteStmt struct {
	BaseStmt
	With      *WithClause
	Table     *TableRef
	Using     []Node
	Where     Node
	Returning *ReturningClause
}

func (n *DeleteStmt) Kind() Kind { return KindDeleteStmt }
func (n *DeleteStmt) Children() []Node {
	return nodes(n.With, n.Table, n.Using, n.Where, n.Returning)
}
func (n *DeleteStmt) String() string { return "DeleteStmt" }

// OnConflictClause is INSERT's "ON CONFLICT [(Target...)] [WHERE
// TargetWhere] DO NOTHING" or "... DO UPDATE SET UpdateSet... [WHERE
// UpdateWhere]", shared by sqlite3 (UPSERT) and PostgreSQL (which spells the
// same construct identically).
type OnConflictClause struct {
	BaseNode
	Target      []string
	TargetWhere Node
	DoNothing   bool
	UpdateSet   []*SetClause
	UpdateWhere Node
}

func (n *OnConflictClause) Kind() Kind { return KindOnConflictClause }
func (n *OnConflictClause) Children() []Node {
	set := make([]Node, len(n.UpdateSet))
	for i, s := range n.UpdateSet {
		set[i] = s
	}

	return nodes(n.TargetWhere, set, n.UpdateWhere)
}
func (n *OnConflictClause) String() string { return "OnConflictClause" }

// ReturningClause is a trailing "RETURNING Columns..." on INSERT, UPDATE, or
// DELETE, supported by both sqlite3 and PostgreSQL. Columns reuses
// ResultColumn so "RETURNING *" and "RETURNING col AS alias" are represented
// the same way as a SELECT list.
type ReturningClause struct {
	BaseNode
	Columns []*ResultColumn
}

func (n *ReturningClause) Kind() Kind { return KindReturningClause }
func (n *ReturningClause) Children() []Node {
	cols := make([]Node, len(n.Columns))
	for i, c := range n.Columns {
		cols[i] = c
	}

	return nodes(cols)
}
func (n *ReturningClause) String() string { return "ReturningClause" }
