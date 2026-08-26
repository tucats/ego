package sqlparse

import (
	"strconv"

	"github.com/tucats/ego/internal/sqlparse/ast"
)

// This file implements StatementKind and Tables, two read-only helpers that
// answer questions about a parsed statement without reformatting it (that's
// Format's job — see format.go). Both follow the same design principle as
// the rest of this package's helper layer (see the file comment in
// sqlparse.go): their result types are declared here, in sqlparse, as plain
// ints with their own String() methods, so a caller can use them without
// ever importing the ast subpackage.

// StatementKind identifies a statement's primary SQL verb: SELECT, INSERT,
// UPDATE, and so on. There is exactly one StatementKind per concrete
// ast.Statement type (see ast/node.go's "Statement is the marker interface
// implemented by every node that can be the root of a parsed SQL
// statement"). A statement that combines a DDL verb with an embedded
// SELECT — PostgreSQL's "CREATE TABLE t AS SELECT ..." is the one example
// this grammar has — reports only the outer verb, StmtCreateTable, not
// StmtSelect; that's the "primary" in this type's name.
type StatementKind int

const (
	// StmtUnknown is the zero value. New only ever returns a Sqlparse
	// wrapping one of the sealed ast.Statement implementations below (see
	// "An unexported method can seal an interface" in ast/node.go), so in
	// practice StatementKind never actually returns StmtUnknown — it exists
	// as the required fallback for the type switch in StatementKind below,
	// the same way ast.KindInvalid exists as ast.Kind's zero value.
	StmtUnknown StatementKind = iota
	StmtSelect
	StmtInsert
	StmtUpdate
	StmtDelete
	StmtCreateTable
	StmtDropTable
	StmtAlterTable
	StmtCreateIndex
	StmtDropIndex
	StmtCreateView
	StmtDropView
	StmtBegin
	StmtCommit
	StmtRollback
	StmtSavepoint
	StmtRelease
)

// statementKindNames maps each StatementKind to the SQL keyword(s) it
// represents, for String().
var statementKindNames = map[StatementKind]string{
	StmtUnknown:     "UNKNOWN",
	StmtSelect:      "SELECT",
	StmtInsert:      "INSERT",
	StmtUpdate:      "UPDATE",
	StmtDelete:      "DELETE",
	StmtCreateTable: "CREATE TABLE",
	StmtDropTable:   "DROP TABLE",
	StmtAlterTable:  "ALTER TABLE",
	StmtCreateIndex: "CREATE INDEX",
	StmtDropIndex:   "DROP INDEX",
	StmtCreateView:  "CREATE VIEW",
	StmtDropView:    "DROP VIEW",
	StmtBegin:       "BEGIN",
	StmtCommit:      "COMMIT",
	StmtRollback:    "ROLLBACK",
	StmtSavepoint:   "SAVEPOINT",
	StmtRelease:     "RELEASE",
}

// String returns the SQL keyword(s) k represents, e.g. "CREATE TABLE".
func (k StatementKind) String() string {
	if name, ok := statementKindNames[k]; ok {
		return name
	}

	return "StatementKind(" + strconv.Itoa(int(k)) + ")"
}

// StatementKind returns the primary SQL verb of p's parsed statement.
func (p Sqlparse) StatementKind() StatementKind {
	switch p.stmt.(type) {
	case *ast.SelectStmt:
		return StmtSelect
	case *ast.InsertStmt:
		return StmtInsert
	case *ast.UpdateStmt:
		return StmtUpdate
	case *ast.DeleteStmt:
		return StmtDelete
	case *ast.CreateTableStmt:
		return StmtCreateTable
	case *ast.DropTableStmt:
		return StmtDropTable
	case *ast.AlterTableStmt:
		return StmtAlterTable
	case *ast.CreateIndexStmt:
		return StmtCreateIndex
	case *ast.DropIndexStmt:
		return StmtDropIndex
	case *ast.CreateViewStmt:
		return StmtCreateView
	case *ast.DropViewStmt:
		return StmtDropView
	case *ast.BeginStmt:
		return StmtBegin
	case *ast.CommitStmt:
		return StmtCommit
	case *ast.RollbackStmt:
		return StmtRollback
	case *ast.SavepointStmt:
		return StmtSavepoint
	case *ast.ReleaseStmt:
		return StmtRelease
	default:
		// See StmtUnknown's comment: unreachable given today's sealed
		// ast.Statement, kept only as the switch's required fallback.
		return StmtUnknown
	}
}

// UsageMode describes how a TableUsage's table participates in the
// statement.
type UsageMode int

const (
	// UsageRead means rows are read from the table: a SELECT's FROM/JOIN
	// list, or any subquery — a FROM-clause subquery, a correlated
	// scalar subquery, or an IN/EXISTS subquery — found anywhere in the
	// statement.
	UsageRead UsageMode = iota

	// UsageWrite means rows in the table are inserted, updated, or deleted.
	UsageWrite

	// UsageAdmin means the table's (or view's) own definition is created,
	// altered, or dropped, rather than its rows being read or written.
	UsageAdmin
)

// usageModeNames maps each UsageMode to its String() text.
var usageModeNames = map[UsageMode]string{
	UsageRead:  "read",
	UsageWrite: "write",
	UsageAdmin: "admin",
}

// String returns the lower-case name of the usage mode, e.g. "write".
func (u UsageMode) String() string {
	if name, ok := usageModeNames[u]; ok {
		return name
	}

	return "UsageMode(" + strconv.Itoa(int(u)) + ")"
}

// TableUsage is one table reference found in a parsed statement, together
// with how that reference is used. See Tables.
type TableUsage struct {
	// Name is the table's name as written in the source, schema-qualified
	// ("schema.table") when the source qualified it that way.
	Name string

	Usage UsageMode
}

// Tables reports every table referenced by p's parsed statement, together
// with how each reference is used.
//
// The same table can appear more than once, with different usages, if the
// statement uses it that way — e.g. "DELETE FROM t USING t AS old WHERE
// t.id = old.id" reports t once as UsageWrite (the row being deleted) and
// once as UsageRead (the copy of t being joined against to find it).
// Tables reports one entry per reference; it does not deduplicate by name.
//
// Because this package parses syntax only (see the ast package's "Syntax
// only" design goal in ast/node.go), Tables cannot tell a real base table
// from a WITH clause's CTE name or a CREATE VIEW's underlying view name —
// both are reported the same way an ordinary table reference would be,
// using whatever name the SQL text gave them. Likewise, a FOREIGN KEY's
// REFERENCES target (see ast.ColumnReferences and ast.TableForeignKey) is
// metadata about a future constraint, not a table whose rows this
// statement touches, so it is not reported here.
func (p Sqlparse) Tables() []TableUsage {
	var out []TableUsage

	// admin records a DDL statement's own target — the table, view, or
	// index-owning table being created, altered, or dropped.
	admin := func(name string) {
		if name != "" {
			out = append(out, TableUsage{Name: name, Usage: UsageAdmin})
		}
	}

	// write records an INSERT/UPDATE/DELETE's target table.
	write := func(ref *ast.TableRef) {
		if name := tableRefName(ref); name != "" {
			out = append(out, TableUsage{Name: name, Usage: UsageWrite})
		}
	}

	// read walks each given node — a WHERE clause, a FROM list, an INSERT
	// source, whatever the caller below passes it — and records every
	// ast.TableRef found anywhere underneath it as UsageRead. Reusing
	// ast.Walk here, rather than writing a bespoke traversal, is what makes
	// this correct for arbitrarily nested subqueries (a scalar subquery
	// inside a CASE inside a WHERE, for instance) for free: Walk already
	// knows how to reach every node in the tree through Children, so read
	// does not need its own logic for each place a subquery can appear.
	read := func(nodes ...ast.Node) {
		for _, n := range nodes {
			ast.Walk(n, func(node ast.Node) bool {
				if ref, ok := node.(*ast.TableRef); ok {
					out = append(out, TableUsage{Name: tableRefName(ref), Usage: UsageRead})
				}

				return true
			})
		}
	}

	switch s := p.stmt.(type) {
	case *ast.SelectStmt:
		read(s)
	case *ast.InsertStmt:
		write(s.Table)
		read(s.With, s.Source, s.OnConflict, s.Returning)
	case *ast.UpdateStmt:
		write(s.Table)
		read(s.With, s.Where, s.Returning)
		read(s.From...)
	case *ast.DeleteStmt:
		write(s.Table)
		read(s.With, s.Where, s.Returning)
		read(s.Using...)
	case *ast.CreateTableStmt:
		admin(tableRefName(s.Table))
		read(s.AsSelect)
	case *ast.DropTableStmt:
		admin(tableRefName(s.Table))
	case *ast.AlterTableStmt:
		admin(tableRefName(s.Table))
	case *ast.CreateIndexStmt:
		// Table and Schema are plain strings, not a *ast.TableRef (see
		// ast/ddl.go), so they are folded together the same way
		// tableRefName does for the *TableRef-typed cases above.
		admin(qualifiedName(s.Schema, s.Table))
		read(s.Where)
	case *ast.DropIndexStmt:
		// DROP INDEX names only the index, never the table it belongs to
		// (see ast.DropIndexStmt in ast/ddl.go), so there is no table name
		// to report here.
	case *ast.CreateViewStmt:
		admin(qualifiedName(s.Schema, s.Name))
		read(s.Select)
	case *ast.DropViewStmt:
		admin(qualifiedName(s.Schema, s.Name))
	case *ast.BeginStmt, *ast.CommitStmt, *ast.RollbackStmt, *ast.SavepointStmt, *ast.ReleaseStmt:
	}

	return out
}

// tableRefName renders a *ast.TableRef as the schema-qualified name Tables
// reports it under, or "" for a nil ref (a defensive case: every statement
// kind Tables handles above always has its Table field set by the parser,
// but nil is cheap to guard against here rather than assumed away).
func tableRefName(ref *ast.TableRef) string {
	if ref == nil {
		return ""
	}

	return qualifiedName(ref.Schema, ref.Name)
}

// qualifiedName joins a schema and a name with "." when schema is set, or
// returns name unchanged when it is not. Shared by tableRefName above and by
// the DDL statement kinds (CreateIndexStmt, CreateViewStmt, DropViewStmt)
// whose schema/name pair is a plain string field rather than a *ast.TableRef.
func qualifiedName(schema, name string) string {
	if schema != "" {
		return schema + "." + name
	}

	return name
}
