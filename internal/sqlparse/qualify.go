package sqlparse

import (
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/sqlparse/ast"
)

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
//
// It also lets a caller reject a parsed statement outright when it names a
// schema other than the DSN's own -- see RestrictToSchema below. That is a
// separate, stricter step from qualifying a bare name: a DSN with an
// explicitly configured schema is meant to be a sandbox around that one
// schema, so a caller should not be able to reach another schema (e.g.
// PostgreSQL's own pg_catalog) just by spelling it out in raw SQL. A DSN
// with no configured schema (Open defaults it to "public" only for the
// connection itself, not for this check) has no such sandbox and permits
// any explicit schema, matching the server's pre-existing behavior for
// callers who never named a schema at all.

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

// RestrictToSchema reports an error if p's parsed statement explicitly names
// any schema other than schema, for every place a schema can be named (the
// same set of *ast.TableRef and DDL nodes QualifyTables above fills in). A
// reference that names schema itself, or leaves the schema blank, is fine --
// only an explicit, different schema is rejected. Calling this with
// schema == "" is a no-op, matching QualifyTables: an empty schema means the
// DSN has none configured, so there is nothing to restrict against.
//
// Call this before QualifyTables, while an explicit schema the caller wrote
// is still distinguishable from one QualifyTables is about to fill in --
// though in practice the two do not conflict, since QualifyTables only ever
// touches a blank Schema field, which this function always allows.
func (p *Sqlparse) RestrictToSchema(schema string) error {
	if schema == "" {
		return nil
	}

	var badSchema string

	ast.Walk(p.stmt, func(n ast.Node) bool {
		if badSchema != "" {
			return false
		}

		if ref, ok := n.(*ast.TableRef); ok && ref.Schema != "" && ref.Schema != schema {
			badSchema = ref.Schema
		}

		return true
	})

	if badSchema == "" {
		switch s := p.stmt.(type) {
		case *ast.CreateIndexStmt:
			badSchema = disallowedSchema(s.Schema, schema)
		case *ast.DropIndexStmt:
			badSchema = disallowedSchema(s.Schema, schema)
		case *ast.CreateViewStmt:
			badSchema = disallowedSchema(s.Schema, schema)
		case *ast.DropViewStmt:
			badSchema = disallowedSchema(s.Schema, schema)
		}
	}

	if badSchema != "" {
		return errors.New(errors.ErrSQLSchemaRestricted).Context(badSchema)
	}

	return nil
}

// disallowedSchema returns found if it is non-empty and does not match
// allowed, else "". Small helper so RestrictToSchema's DDL switch cases
// above stay one line each, matching QualifyTables' equivalent switch.
func disallowedSchema(found, allowed string) string {
	if found != "" && found != allowed {
		return found
	}

	return ""
}
