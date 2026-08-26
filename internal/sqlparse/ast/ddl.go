package ast

// This file defines the data-definition statements: CREATE/DROP/ALTER
// TABLE, CREATE/DROP INDEX, and CREATE/DROP VIEW, plus the column- and
// table-level constraint nodes used inside a CreateTableStmt.

// CreateTableStmt is "CREATE [TEMP] TABLE [IF NOT EXISTS] Table
// (Columns..., Constraints...) [WITHOUT ROWID]", or the PostgreSQL
// "CREATE TABLE Table AS Select" form, in which case AsSelect is set and
// Columns/Constraints are empty.
type CreateTableStmt struct {
	BaseStmt
	Temp         bool
	IfNotExists  bool
	Table        *TableRef
	Columns      []*ColumnDef
	Constraints  []Node
	WithoutRowID bool
	AsSelect     Node
}

func (n *CreateTableStmt) Kind() Kind { return KindCreateTableStmt }
func (n *CreateTableStmt) Children() []Node {
	cols := make([]Node, len(n.Columns))
	for i, c := range n.Columns {
		cols[i] = c
	}

	return nodes(n.Table, cols, n.Constraints, n.AsSelect)
}
func (n *CreateTableStmt) String() string { return "CreateTableStmt" }

// ColumnDef is one column definition inside a CREATE TABLE's column list.
// Type is nil for sqlite3's optional (type-less) column form. Constraints
// holds zero or more of *ColumnPrimaryKey, *ColumnNotNull, *ColumnUnique,
// *ColumnCheck, *ColumnDefault, *ColumnReferences, *ColumnCollate, and
// *ColumnGenerated, in source order.
type ColumnDef struct {
	BaseNode
	Name        string
	Type        *TypeName
	Constraints []Node
}

func (n *ColumnDef) Kind() Kind       { return KindColumnDef }
func (n *ColumnDef) Children() []Node { return nodes(n.Type, n.Constraints) }
func (n *ColumnDef) String() string   { return "ColumnDef(" + n.Name + ")" }

// ColumnPrimaryKey is a column-level "[CONSTRAINT Name] PRIMARY KEY [ASC|DESC]
// [AUTOINCREMENT] [ON CONFLICT Conflict]".
type ColumnPrimaryKey struct {
	BaseNode
	Name          string
	Desc          bool
	AutoIncrement bool
	Conflict      string
}

func (n *ColumnPrimaryKey) Kind() Kind       { return KindColumnPrimaryKey }
func (n *ColumnPrimaryKey) Children() []Node { return nil }
func (n *ColumnPrimaryKey) String() string   { return "ColumnPrimaryKey" }

// ColumnNotNull is a column-level "[CONSTRAINT Name] NOT NULL [ON CONFLICT
// Conflict]".
type ColumnNotNull struct {
	BaseNode
	Name     string
	Conflict string
}

func (n *ColumnNotNull) Kind() Kind       { return KindColumnNotNull }
func (n *ColumnNotNull) Children() []Node { return nil }
func (n *ColumnNotNull) String() string   { return "ColumnNotNull" }

// ColumnUnique is a column-level "[CONSTRAINT Name] UNIQUE [ON CONFLICT
// Conflict]".
type ColumnUnique struct {
	BaseNode
	Name     string
	Conflict string
}

func (n *ColumnUnique) Kind() Kind       { return KindColumnUnique }
func (n *ColumnUnique) Children() []Node { return nil }
func (n *ColumnUnique) String() string   { return "ColumnUnique" }

// ColumnCheck is a column-level "[CONSTRAINT Name] CHECK (Expr)".
type ColumnCheck struct {
	BaseNode
	Name string
	Expr Node
}

func (n *ColumnCheck) Kind() Kind       { return KindColumnCheck }
func (n *ColumnCheck) Children() []Node { return nodes(n.Expr) }
func (n *ColumnCheck) String() string   { return "ColumnCheck" }

// ColumnDefault is a column-level "DEFAULT Value", where Value is either a
// literal, a signed number, or a parenthesized expression.
type ColumnDefault struct {
	BaseNode
	Value Node
}

func (n *ColumnDefault) Kind() Kind       { return KindColumnDefault }
func (n *ColumnDefault) Children() []Node { return nodes(n.Value) }
func (n *ColumnDefault) String() string   { return "ColumnDefault" }

// ColumnReferences is a column-level "[CONSTRAINT Name] REFERENCES Table
// (Columns...) [ON DELETE OnDelete] [ON UPDATE OnUpdate] [[NOT] DEFERRABLE
// [INITIALLY Initially]]". OnDelete and OnUpdate hold the normalized action
// text ("CASCADE", "SET NULL", "SET DEFAULT", "RESTRICT", "NO ACTION") and
// are empty when not written. Deferrable is "DEFERRABLE", "NOT DEFERRABLE",
// or "" when not written; Initially is "DEFERRED", "IMMEDIATE", or "".
type ColumnReferences struct {
	BaseNode
	Name       string
	Table      string
	Columns    []string
	OnDelete   string
	OnUpdate   string
	Deferrable string
	Initially  string
}

func (n *ColumnReferences) Kind() Kind       { return KindColumnReferences }
func (n *ColumnReferences) Children() []Node { return nil }
func (n *ColumnReferences) String() string   { return "ColumnReferences(" + n.Table + ")" }

// ColumnCollate is a column-level "COLLATE Collation".
type ColumnCollate struct {
	BaseNode
	Collation string
}

func (n *ColumnCollate) Kind() Kind       { return KindColumnCollate }
func (n *ColumnCollate) Children() []Node { return nil }
func (n *ColumnCollate) String() string   { return "ColumnCollate(" + n.Collation + ")" }

// ColumnGenerated is a column-level "[GENERATED ALWAYS] AS (Expr) [STORED |
// VIRTUAL]".
type ColumnGenerated struct {
	BaseNode
	Expr   Node
	Stored bool
}

func (n *ColumnGenerated) Kind() Kind       { return KindColumnGenerated }
func (n *ColumnGenerated) Children() []Node { return nodes(n.Expr) }
func (n *ColumnGenerated) String() string   { return "ColumnGenerated" }

// TablePrimaryKey is a table-level "[CONSTRAINT Name] PRIMARY KEY
// (Columns...) [ON CONFLICT Conflict]".
type TablePrimaryKey struct {
	BaseNode
	Name     string
	Columns  []*OrderByTerm
	Conflict string
}

func (n *TablePrimaryKey) Kind() Kind { return KindTablePrimaryKey }
func (n *TablePrimaryKey) Children() []Node {
	cols := make([]Node, len(n.Columns))
	for i, c := range n.Columns {
		cols[i] = c
	}

	return nodes(cols)
}
func (n *TablePrimaryKey) String() string { return "TablePrimaryKey" }

// TableUnique is a table-level "[CONSTRAINT Name] UNIQUE (Columns...) [ON
// CONFLICT Conflict]".
type TableUnique struct {
	BaseNode
	Name     string
	Columns  []*OrderByTerm
	Conflict string
}

func (n *TableUnique) Kind() Kind { return KindTableUnique }
func (n *TableUnique) Children() []Node {
	cols := make([]Node, len(n.Columns))
	for i, c := range n.Columns {
		cols[i] = c
	}

	return nodes(cols)
}
func (n *TableUnique) String() string { return "TableUnique" }

// TableForeignKey is a table-level "[CONSTRAINT Name] FOREIGN KEY
// (Columns...) REFERENCES RefTable (RefColumns...) [ON DELETE OnDelete] [ON
// UPDATE OnUpdate] [[NOT] DEFERRABLE [INITIALLY Initially]]". See
// ColumnReferences for the meaning of Deferrable and Initially.
type TableForeignKey struct {
	BaseNode
	Name       string
	Columns    []string
	RefTable   string
	RefColumns []string
	OnDelete   string
	OnUpdate   string
	Deferrable string
	Initially  string
}

func (n *TableForeignKey) Kind() Kind       { return KindTableForeignKey }
func (n *TableForeignKey) Children() []Node { return nil }
func (n *TableForeignKey) String() string   { return "TableForeignKey(" + n.RefTable + ")" }

// TableCheck is a table-level "[CONSTRAINT Name] CHECK (Expr)".
type TableCheck struct {
	BaseNode
	Name string
	Expr Node
}

func (n *TableCheck) Kind() Kind       { return KindTableCheck }
func (n *TableCheck) Children() []Node { return nodes(n.Expr) }
func (n *TableCheck) String() string   { return "TableCheck" }

// DropTableStmt is "DROP TABLE [IF EXISTS] Table [CASCADE | RESTRICT]".
// Cascade and Restrict are PostgreSQL-only and mutually exclusive.
type DropTableStmt struct {
	BaseStmt
	IfExists bool
	Table    *TableRef
	Cascade  bool
	Restrict bool
}

func (n *DropTableStmt) Kind() Kind       { return KindDropTableStmt }
func (n *DropTableStmt) Children() []Node { return nodes(n.Table) }
func (n *DropTableStmt) String() string   { return "DropTableStmt" }

// AlterTableStmt is "ALTER TABLE Table Action", where Action is one of
// *AddColumn, *DropColumn, *RenameColumn, or *RenameTable.
type AlterTableStmt struct {
	BaseStmt
	Table  *TableRef
	Action Node
}

func (n *AlterTableStmt) Kind() Kind       { return KindAlterTableStmt }
func (n *AlterTableStmt) Children() []Node { return nodes(n.Table, n.Action) }
func (n *AlterTableStmt) String() string   { return "AlterTableStmt" }

// AddColumn is ALTER TABLE's "ADD [COLUMN] Column".
type AddColumn struct {
	BaseNode
	Column *ColumnDef
}

func (n *AddColumn) Kind() Kind       { return KindAddColumn }
func (n *AddColumn) Children() []Node { return nodes(n.Column) }
func (n *AddColumn) String() string   { return "AddColumn" }

// DropColumn is ALTER TABLE's "DROP [COLUMN] Name".
type DropColumn struct {
	BaseNode
	Name string
}

func (n *DropColumn) Kind() Kind       { return KindDropColumn }
func (n *DropColumn) Children() []Node { return nil }
func (n *DropColumn) String() string   { return "DropColumn(" + n.Name + ")" }

// RenameColumn is ALTER TABLE's "RENAME [COLUMN] From TO To".
type RenameColumn struct {
	BaseNode
	From string
	To   string
}

func (n *RenameColumn) Kind() Kind       { return KindRenameColumn }
func (n *RenameColumn) Children() []Node { return nil }
func (n *RenameColumn) String() string   { return "RenameColumn(" + n.From + " -> " + n.To + ")" }

// RenameTable is ALTER TABLE's "RENAME TO To".
type RenameTable struct {
	BaseNode
	To string
}

func (n *RenameTable) Kind() Kind       { return KindRenameTable }
func (n *RenameTable) Children() []Node { return nil }
func (n *RenameTable) String() string   { return "RenameTable(" + n.To + ")" }

// CreateIndexStmt is "CREATE [UNIQUE] INDEX [IF NOT EXISTS] Name ON
// [Schema.]Table (Columns...) [WHERE Where]". Columns reuses OrderByTerm
// since an index column has the same shape as an ORDER BY term (an
// expression with an optional collation and sort direction) and PostgreSQL
// additionally allows expression indexes. Schema qualifies Table, not Name --
// PostgreSQL creates the index in its table's own schema and does not accept
// a schema prefix on the index name itself.
type CreateIndexStmt struct {
	BaseStmt
	Unique      bool
	IfNotExists bool
	Name        string
	Schema      string
	Table       string
	Columns     []*OrderByTerm
	Where       Node
}

func (n *CreateIndexStmt) Kind() Kind { return KindCreateIndexStmt }
func (n *CreateIndexStmt) Children() []Node {
	cols := make([]Node, len(n.Columns))
	for i, c := range n.Columns {
		cols[i] = c
	}

	return nodes(cols, n.Where)
}
func (n *CreateIndexStmt) String() string { return "CreateIndexStmt(" + n.Name + ")" }

// DropIndexStmt is "DROP INDEX [IF EXISTS] [Schema.]Name".
type DropIndexStmt struct {
	BaseStmt
	IfExists bool
	Schema   string
	Name     string
}

func (n *DropIndexStmt) Kind() Kind       { return KindDropIndexStmt }
func (n *DropIndexStmt) Children() []Node { return nil }
func (n *DropIndexStmt) String() string   { return "DropIndexStmt(" + n.Name + ")" }

// CreateViewStmt is "CREATE [OR REPLACE] [TEMP] VIEW [IF NOT EXISTS]
// [Schema.]Name [(Columns...)] AS Select". OrReplace is PostgreSQL-only.
type CreateViewStmt struct {
	BaseStmt
	OrReplace   bool
	Temp        bool
	IfNotExists bool
	Name        string
	Schema      string
	Columns     []string
	Select      Node
}

func (n *CreateViewStmt) Kind() Kind       { return KindCreateViewStmt }
func (n *CreateViewStmt) Children() []Node { return nodes(n.Select) }
func (n *CreateViewStmt) String() string   { return "CreateViewStmt(" + n.Name + ")" }

// DropViewStmt is "DROP VIEW [IF EXISTS] [Schema.]Name [CASCADE | RESTRICT]".
type DropViewStmt struct {
	BaseStmt
	IfExists bool
	Schema   string
	Name     string
	Cascade  bool
	Restrict bool
}

func (n *DropViewStmt) Kind() Kind       { return KindDropViewStmt }
func (n *DropViewStmt) Children() []Node { return nil }
func (n *DropViewStmt) String() string   { return "DropViewStmt(" + n.Name + ")" }
