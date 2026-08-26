package sqlparse

import (
	"strconv"

	"github.com/tucats/ego/internal/sqlparse/ast"
)

// This file formats CREATE/DROP/ALTER TABLE, CREATE/DROP INDEX, and
// CREATE/DROP VIEW — the counterpart to ddl.go.

func (pr *printer) createTableStmt(s *ast.CreateTableStmt) {
	pr.write("CREATE ")

	if s.Temp {
		pr.write("TEMP ")
	}

	pr.write("TABLE ")

	if s.IfNotExists {
		pr.write("IF NOT EXISTS ")
	}

	pr.tableRef(s.Table)

	if s.AsSelect != nil {
		pr.write(" AS")
		pr.newline()
		pr.selectLike(s.AsSelect)

		return
	}

	pr.write(" (")
	pr.indent()

	// Columns are emitted before constraints regardless of their original
	// order in the source — see "Known limitations" in format.go's file
	// comment for why.
	entries := 0

	for _, c := range s.Columns {
		if entries > 0 {
			pr.write(",")
		}

		pr.newline()
		pr.columnDef(c)

		entries++
	}

	for _, c := range s.Constraints {
		if entries > 0 {
			pr.write(",")
		}

		pr.newline()
		pr.tableConstraint(c)

		entries++
	}

	pr.dedent()
	pr.newline()
	pr.write(")")

	if s.WithoutRowID {
		pr.write(" WITHOUT ROWID")
	}
}

func (pr *printer) columnDef(c *ast.ColumnDef) {
	pr.ident(c.Name)

	if c.Type != nil {
		pr.write(" ")
		pr.typeName(c.Type)
	}

	for _, con := range c.Constraints {
		pr.write(" ")
		pr.columnConstraint(con)
	}
}

func (pr *printer) typeName(t *ast.TypeName) {
	pr.write(t.Name)

	if len(t.Args) > 0 {
		pr.write("(")

		for i, a := range t.Args {
			if i > 0 {
				pr.write(", ")
			}

			pr.write(strconv.Itoa(a))
		}

		pr.write(")")
	}
}

// constraintPrefix formats the optional "CONSTRAINT name " that can precede
// any column- or table-level constraint, shared by both kinds below.
func (pr *printer) constraintPrefix(name string) {
	if name != "" {
		pr.write("CONSTRAINT ")
		pr.ident(name)
		pr.write(" ")
	}
}

// conflictActionClause formats sqlite3's optional trailing "ON CONFLICT
// action" on a PRIMARY KEY/NOT NULL/UNIQUE constraint (see
// parseOptionalConflictClause in ddl.go) — distinct from INSERT's
// statement-level ON CONFLICT clause (onConflictClause in format_dml.go).
func (pr *printer) conflictActionClause(action string) {
	if action != "" {
		pr.write(" ON CONFLICT ")
		pr.write(action)
	}
}

// referentialActions formats the shared "[ON DELETE action] [ON UPDATE
// action] [[NOT] DEFERRABLE [INITIALLY ...]]" tail of a REFERENCES clause,
// whether it came from a column-level ColumnReferences or a table-level
// TableForeignKey — see parseReferentialActions/parseDeferrableClause in
// ddl.go, which both constraint kinds' parsing shares the same way.
func (pr *printer) referentialActions(onDelete, onUpdate, deferrable, initially string) {
	if onDelete != "" {
		pr.write(" ON DELETE ")
		pr.write(onDelete)
	}

	if onUpdate != "" {
		pr.write(" ON UPDATE ")
		pr.write(onUpdate)
	}

	if deferrable != "" {
		pr.write(" ")
		pr.write(deferrable)

		if initially != "" {
			pr.write(" INITIALLY ")
			pr.write(initially)
		}
	}
}

// columnConstraint formats one column-level constraint. columnConstraintFollows
// in ddl.go guarantees the parser only ever builds one of these eight
// concrete types, so there is no default/fallback case needed here the way
// there is in, say, statement (format.go), which has to account for
// external Node implementations.
func (pr *printer) columnConstraint(n ast.Node) {
	switch v := n.(type) {
	case *ast.ColumnPrimaryKey:
		pr.constraintPrefix(v.Name)
		pr.write("PRIMARY KEY")

		if v.Desc {
			pr.write(" DESC")
		}

		if v.AutoIncrement {
			pr.write(" AUTOINCREMENT")
		}

		pr.conflictActionClause(v.Conflict)
	case *ast.ColumnNotNull:
		pr.constraintPrefix(v.Name)
		pr.write("NOT NULL")
		pr.conflictActionClause(v.Conflict)
	case *ast.ColumnUnique:
		pr.constraintPrefix(v.Name)
		pr.write("UNIQUE")
		pr.conflictActionClause(v.Conflict)
	case *ast.ColumnCheck:
		pr.constraintPrefix(v.Name)
		pr.write("CHECK (")
		pr.expr(v.Expr)
		pr.write(")")
	case *ast.ColumnDefault:
		pr.write("DEFAULT ")
		pr.expr(v.Value)
	case *ast.ColumnReferences:
		pr.constraintPrefix(v.Name)
		pr.write("REFERENCES ")
		// v.Table is a pre-joined "schema.table" string — see "Known
		// limitations" in format.go's file comment for why this is
		// written as-is rather than through ident.
		pr.write(v.Table)

		if len(v.Columns) > 0 {
			pr.write(" (")

			for i, c := range v.Columns {
				if i > 0 {
					pr.write(", ")
				}

				pr.ident(c)
			}

			pr.write(")")
		}

		pr.referentialActions(v.OnDelete, v.OnUpdate, v.Deferrable, v.Initially)
	case *ast.ColumnCollate:
		pr.write("COLLATE ")
		pr.ident(v.Collation)
	case *ast.ColumnGenerated:
		pr.write("GENERATED ALWAYS AS (")
		pr.expr(v.Expr)
		pr.write(")")

		if v.Stored {
			pr.write(" STORED")
		} else {
			pr.write(" VIRTUAL")
		}
	}
}

// tableConstraint formats one table-level constraint — the counterpart to
// columnConstraint above, for the four kinds tableConstraintFollows (ddl.go)
// recognizes.
func (pr *printer) tableConstraint(n ast.Node) {
	switch v := n.(type) {
	case *ast.TablePrimaryKey:
		pr.constraintPrefix(v.Name)
		pr.write("PRIMARY KEY (")
		pr.indexColumnList(v.Columns)
		pr.write(")")
		pr.conflictActionClause(v.Conflict)
	case *ast.TableUnique:
		pr.constraintPrefix(v.Name)
		pr.write("UNIQUE (")
		pr.indexColumnList(v.Columns)
		pr.write(")")
		pr.conflictActionClause(v.Conflict)
	case *ast.TableForeignKey:
		pr.constraintPrefix(v.Name)
		pr.write("FOREIGN KEY (")

		for i, c := range v.Columns {
			if i > 0 {
				pr.write(", ")
			}

			pr.ident(c)
		}

		pr.write(") REFERENCES ")
		// Same caveat as ColumnReferences.Table above: RefTable is a
		// pre-joined "schema.table" string, written as-is.
		pr.write(v.RefTable)

		if len(v.RefColumns) > 0 {
			pr.write(" (")

			for i, c := range v.RefColumns {
				if i > 0 {
					pr.write(", ")
				}

				pr.ident(c)
			}

			pr.write(")")
		}

		pr.referentialActions(v.OnDelete, v.OnUpdate, v.Deferrable, v.Initially)
	case *ast.TableCheck:
		pr.constraintPrefix(v.Name)
		pr.write("CHECK (")
		pr.expr(v.Expr)
		pr.write(")")
	}
}

func (pr *printer) dropTableStmt(s *ast.DropTableStmt) {
	pr.write("DROP TABLE ")

	if s.IfExists {
		pr.write("IF EXISTS ")
	}

	pr.tableRef(s.Table)

	switch {
	case s.Cascade:
		pr.write(" CASCADE")
	case s.Restrict:
		pr.write(" RESTRICT")
	}
}

func (pr *printer) alterTableStmt(s *ast.AlterTableStmt) {
	pr.write("ALTER TABLE ")
	pr.tableRef(s.Table)
	pr.write(" ")
	pr.alterTableAction(s.Action)
}

func (pr *printer) alterTableAction(n ast.Node) {
	switch v := n.(type) {
	case *ast.AddColumn:
		pr.write("ADD COLUMN ")
		pr.columnDef(v.Column)
	case *ast.DropColumn:
		pr.write("DROP COLUMN ")
		pr.ident(v.Name)
	case *ast.RenameColumn:
		pr.write("RENAME COLUMN ")
		pr.ident(v.From)
		pr.write(" TO ")
		pr.ident(v.To)
	case *ast.RenameTable:
		pr.write("RENAME TO ")
		pr.ident(v.To)
	}
}

func (pr *printer) createIndexStmt(s *ast.CreateIndexStmt) {
	pr.write("CREATE ")

	if s.Unique {
		pr.write("UNIQUE ")
	}

	pr.write("INDEX ")

	if s.IfNotExists {
		pr.write("IF NOT EXISTS ")
	}

	pr.ident(s.Name)
	pr.write(" ON ")

	if s.Schema != "" {
		pr.ident(s.Schema)
		pr.write(".")
	}

	pr.ident(s.Table)
	pr.write(" (")
	pr.indexColumnList(s.Columns)
	pr.write(")")

	if s.Where != nil {
		pr.newline()
		pr.write("WHERE ")
		pr.expr(s.Where)
	}
}

func (pr *printer) dropIndexStmt(s *ast.DropIndexStmt) {
	pr.write("DROP INDEX ")

	if s.IfExists {
		pr.write("IF EXISTS ")
	}

	if s.Schema != "" {
		pr.ident(s.Schema)
		pr.write(".")
	}

	pr.ident(s.Name)
}

func (pr *printer) createViewStmt(s *ast.CreateViewStmt) {
	pr.write("CREATE ")

	if s.OrReplace {
		pr.write("OR REPLACE ")
	}

	if s.Temp {
		pr.write("TEMP ")
	}

	pr.write("VIEW ")

	if s.IfNotExists {
		pr.write("IF NOT EXISTS ")
	}

	if s.Schema != "" {
		pr.ident(s.Schema)
		pr.write(".")
	}

	pr.ident(s.Name)

	if len(s.Columns) > 0 {
		pr.write(" (")

		for i, c := range s.Columns {
			if i > 0 {
				pr.write(", ")
			}

			pr.ident(c)
		}

		pr.write(")")
	}

	pr.write(" AS")
	pr.newline()
	pr.selectLike(s.Select)
}

func (pr *printer) dropViewStmt(s *ast.DropViewStmt) {
	pr.write("DROP VIEW ")

	if s.IfExists {
		pr.write("IF EXISTS ")
	}

	if s.Schema != "" {
		pr.ident(s.Schema)
		pr.write(".")
	}

	pr.ident(s.Name)

	switch {
	case s.Cascade:
		pr.write(" CASCADE")
	case s.Restrict:
		pr.write(" RESTRICT")
	}
}
