package sqlparse

import (
	"strconv"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/sqlparse/ast"
)

// This file is the entry point for this package's higher-level helper
// layer, as opposed to the lower-level Parse function in parser.go. Where
// Parse just turns source text into an AST and hands it back, New wraps
// that AST in a Sqlparse value that carries its dialect along with it, so
// later helpers (Format below, and others expected to join it over time —
// see the file comment in format.go) don't need the dialect passed to them
// separately every time.

// SQLite and PostgreSQL are the two dialect values New accepts. They are
// plain ints, matching ast.DialectSQLite and ast.DialectPostgreSQL
// one-for-one, so that a caller of this package's helper layer can select
// a dialect without importing the ast subpackage at all — everything
// needed for basic use lives in this one package.
const (
	SQLite     = int(ast.DialectSQLite)
	PostgreSQL = int(ast.DialectPostgreSQL)
)

// Sqlparse holds a single parsed SQL statement together with the dialect it
// was parsed as. It is returned by New, and is the receiver for this
// package's higher-level helpers that work from the parsed result rather
// than from raw source text.
type Sqlparse struct {
	stmt    ast.Statement
	dialect ast.Dialect
}

// New parses sqlText as a single SQL statement in the given dialect (SQLite
// or PostgreSQL, above) and returns a Sqlparse wrapping the result. Any
// error from the parse — a syntax error located by line and column, or an
// unrecognized dialect value — is returned here rather than deferred to a
// later call.
func New(sqlText string, dialect int) (*Sqlparse, error) {
	d, err := dialectFromInt(dialect)
	if err != nil {
		return nil, err
	}

	stmt, err := Parse(sqlText, d)
	if err != nil {
		return nil, err
	}

	return &Sqlparse{stmt: stmt, dialect: d}, nil
}

// dialectFromInt validates and converts a caller-supplied dialect constant
// to the ast package's own Dialect type. It exists (rather than a bare
// ast.Dialect(dialect) conversion) so that a caller passing an out-of-range
// int — a typo'd constant, or a value from an unrelated enum — gets a clear
// error here instead of a Sqlparse silently carrying a meaningless dialect
// that only manifests as strange behavior later, in Format.
func dialectFromInt(dialect int) (ast.Dialect, error) {
	switch dialect {
	case SQLite:
		return ast.DialectSQLite, nil
	case PostgreSQL:
		return ast.DialectPostgreSQL, nil
	default:
		return 0, errors.New(errors.ErrSQLInvalidDialect).Context(strconv.Itoa(dialect))
	}
}

// Statement returns the root of the parsed statement's AST, for callers
// that want to inspect or walk the tree themselves — with ast.Walk, or a
// type switch over the concrete statement types in the ast package — rather
// than go through a higher-level helper like Format.
func (p Sqlparse) Statement() ast.Statement {
	return p.stmt
}
