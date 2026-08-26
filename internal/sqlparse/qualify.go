package sqlparse

import "github.com/tucats/ego/internal/sqlparse/ast"

// This file lets a caller rewrite a parsed statement so that every table
// reference it left unqualified gets an explicit schema, before formatting
// it back to text. It exists so that raw SQL sent to the server's @sql and
// @transaction endpoints can be pinned to the requesting DSN's own schema --
// the same schema the structured /rows and /tables endpoints always inject
// (see internal/server/tables/parsing.FullName) -- rather than resolving
// through whatever default schema (e.g. PostgreSQL's search_path) the
// database connection happens to have. See internal/server/tables/database's
// Open, which already resolves a DSN's configured schema this way for every
// other query-building code path in the server.

// QualifyTables rewrites every table reference in p's parsed statement that
// was NOT already schema-qualified in the source text to use schema instead.
// A reference the caller wrote with its own schema (e.g. "other.table") is
// left untouched -- only a bare, unqualified name is filled in. Calling this
// with schema == "" is a no-op, since there is then nothing to fill in.
//
// This covers every statement kind that names a table or view:
//   - Every *ast.TableRef anywhere in the tree (SELECT FROM/JOIN sources,
//     subqueries, CTEs, and the INSERT/UPDATE/DELETE/CREATE TABLE/DROP
//     TABLE/ALTER TABLE target) is reached generically via ast.Walk, since
//     TableRef participates in Children() everywhere it appears.
//   - CREATE INDEX's "ON table" and DROP INDEX's "[schema.]name" and CREATE
//     VIEW/DROP VIEW's "[schema.]name" are plain string fields on the
//     statement itself, not *ast.TableRef nodes, so they are not reached by
//     Walk and are qualified explicitly below instead.
//
// Call this after any authorization check that needs to see which schema the
// source text itself named (this rewrites that information away) but before
// Format, since Format is what actually consults the Schema fields this sets.
func (p *Sqlparse) QualifyTables(schema string) {
	if schema == "" {
		return
	}

	ast.Walk(p.stmt, func(n ast.Node) bool {
		if ref, ok := n.(*ast.TableRef); ok && ref.Schema == "" {
			ref.Schema = schema
		}

		return true
	})

	switch s := p.stmt.(type) {
	case *ast.CreateIndexStmt:
		if s.Schema == "" {
			s.Schema = schema
		}
	case *ast.DropIndexStmt:
		if s.Schema == "" {
			s.Schema = schema
		}
	case *ast.CreateViewStmt:
		if s.Schema == "" {
			s.Schema = schema
		}
	case *ast.DropViewStmt:
		if s.Schema == "" {
			s.Schema = schema
		}
	}
}
