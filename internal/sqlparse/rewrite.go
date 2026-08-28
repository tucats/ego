package sqlparse

import (
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/sqlparse/ast"
)

// This file implements Rewrite, which normalizes the handful of DDL/DML
// constructs where sqlite3 and PostgreSQL disagree syntactically for the
// same intent — generated/auto-incrementing primary keys, sqlite3's WITHOUT
// ROWID table option, and sqlite3's "INSERT OR ..." conflict-resolution
// shorthand — so a client can write SQL using either dialect's idioms and
// have it run correctly no matter which one Sqlparse was actually created
// for (see New's dialect parameter). This lets a caller of the @sql
// endpoint (internal/server/tables/sql_permissions.go, which calls Rewrite
// on every statement it parses, right before Format) stay ignorant of
// which DSN provider its SQL will ultimately run against.
//
// Two constructs are deliberately left alone because both dialects already
// accept the same syntax for them: PostgreSQL-style "INSERT ... ON CONFLICT
// (...) DO UPDATE/DO NOTHING" upserts — which sqlite3 has also accepted
// since 3.24.0 — and "CREATE TABLE ... AS SELECT", which both dialects
// parse and execute identically. Only sqlite3's older "INSERT OR ..." form
// needs translating, and only toward PostgreSQL, which has no equivalent
// syntax at all.

// UniqueKeyLookup resolves the column(s) that make up a table's primary key
// or a single-column unique index, keyed by table name. Rewrite consults it
// only when translating sqlite3's "INSERT OR REPLACE" toward PostgreSQL —
// see rewriteInsert — since this package has no database access of its own
// (it only parses and formats text) and so cannot look this up itself.
// Passing nil is fine for any call where that specific combination doesn't
// arise; Rewrite reports a clear error rather than a nil-pointer panic if
// it turns out to be needed and wasn't supplied.
type UniqueKeyLookup func(table string) ([]string, error)

// Rewrite normalizes the statement's dialect-specific constructs, in place,
// to match p's own dialect (the one it was parsed with — see New). It
// returns one human-readable note per change actually made, meant for
// logging (see the @sql endpoint's use of it) rather than for the caller to
// act on, and a non-nil error if some construct in the statement has no
// faithful equivalent in the target dialect — see the "Known limitation"
// notes on rewriteColumnGenerated and rewriteInsert below. On error, the
// statement may have been partially rewritten; a caller must not format or
// execute it.
func (p *Sqlparse) Rewrite(uniqueKey UniqueKeyLookup) ([]string, error) {
	switch s := p.stmt.(type) {
	case *ast.CreateTableStmt:
		return rewriteCreateTable(s, p.dialect)
	case *ast.AlterTableStmt:
		if add, ok := s.Action.(*ast.AddColumn); ok {
			notes, _, err := rewriteColumnGenerated(add.Column, false, p.dialect)

			return notes, err
		}
	case *ast.InsertStmt:
		return rewriteInsert(s, p.dialect, uniqueKey)
	}

	return nil, nil
}

// rewriteCreateTable applies rewriteColumnGenerated to every column of s,
// then drops sqlite3's WITHOUT ROWID when targeting PostgreSQL (which has
// no rowid concept, so there's nothing to translate it to — it's simply
// removed, since it only ever affects sqlite3's own storage layout, never a
// query's result).
func rewriteCreateTable(s *ast.CreateTableStmt, dialect ast.Dialect) ([]string, error) {
	if s.AsSelect != nil {
		return nil, nil
	}

	var notes []string

	// A table-level PRIMARY KEY naming exactly one column is the other
	// place (besides a column-level PRIMARY KEY constraint) a generated-key
	// column's PRIMARY KEY-ness can be written — see rewriteColumnGenerated.
	tablePKIndex := -1
	tablePKColumn := ""

	for i, c := range s.Constraints {
		if tpk, ok := c.(*ast.TablePrimaryKey); ok {
			tablePKIndex = i

			if len(tpk.Columns) == 1 {
				if cr, ok := tpk.Columns[0].Expr.(*ast.ColumnRef); ok {
					tablePKColumn = cr.Column
				}
			}

			break
		}
	}

	for _, col := range s.Columns {
		colNotes, promote, err := rewriteColumnGenerated(col, col.Name == tablePKColumn && tablePKColumn != "", dialect)
		if err != nil {
			return nil, errors.New(err).Context(s.Table.Name + "." + col.Name)
		}

		notes = append(notes, colNotes...)

		if promote && tablePKIndex >= 0 {
			s.Constraints = append(s.Constraints[:tablePKIndex], s.Constraints[tablePKIndex+1:]...)
			notes = append(notes, "moved single-column PRIMARY KEY for \""+col.Name+"\" from table level to column level, required by sqlite3's AUTOINCREMENT")
			tablePKIndex = -1
		}
	}

	if dialect == ast.DialectPostgreSQL && s.WithoutRowID {
		s.WithoutRowID = false

		notes = append(notes, "dropped WITHOUT ROWID (sqlite3-only; PostgreSQL has no equivalent or need for it)")
	}

	return notes, nil
}

// integerWidthAliases maps common integer type-name spellings to the
// canonical width bucket Rewrite uses when choosing a PostgreSQL identity
// column's type. Any type name not listed here — including no declared
// type at all, which sqlite3 permits — defaults to INTEGER in
// pgIntegerWidth, matching sqlite3's own single, unsized integer storage
// class.
var integerWidthAliases = map[string]string{
	"SMALLINT": "SMALLINT", "INT2": "SMALLINT",
	"INTEGER": "INTEGER", "INT": "INTEGER", "INT4": "INTEGER",
	"BIGINT": "BIGINT", "INT8": "BIGINT",
}

func pgIntegerWidth(t *ast.TypeName) string {
	if t != nil {
		if w, ok := integerWidthAliases[t.Name]; ok {
			return w
		}
	}

	return "INTEGER"
}

// rewriteColumnGenerated normalizes col's generated-key syntax — sqlite3's
// PRIMARY KEY AUTOINCREMENT (ast.ColumnPrimaryKey.AutoIncrement), or
// PostgreSQL's GENERATED ... AS IDENTITY (ast.ColumnIdentity; the
// SERIAL/BIGSERIAL/SMALLSERIAL pseudo-types have already been desugared to
// this by the parser — see parseColumnDef in ddl.go) — to match dialect.
// hasMatchingTablePK reports whether col is exactly the one column named by
// its CREATE TABLE's table-level PRIMARY KEY constraint, if any (always
// false from the ALTER TABLE ADD COLUMN caller, which has no such
// constraint to consult). It returns true as its second result when the
// caller must now remove that table-level constraint, because this call
// moved it onto the column itself — the only place sqlite3's AUTOINCREMENT
// is legal.
//
// Known limitation: a generated-key column that is not also its table's
// sole primary-key column — an identity column used as a surrogate value
// while some other column is the actual PRIMARY KEY, which PostgreSQL
// allows and sqlite3 does not — cannot be translated toward sqlite3: there
// is no sqlite3 syntax for "auto-increment this column" independent of
// "this is the table's PRIMARY KEY". Rewrite reports
// errors.ErrSQLDialectRewrite for that combination rather than silently
// dropping the auto-increment behavior or emitting SQL sqlite3 will reject.
func rewriteColumnGenerated(col *ast.ColumnDef, hasMatchingTablePK bool, dialect ast.Dialect) (notes []string, promoteTablePK bool, err error) {
	var (
		pk       *ast.ColumnPrimaryKey
		identity *ast.ColumnIdentity
		identIdx = -1
	)

	for i, c := range col.Constraints {
		switch v := c.(type) {
		case *ast.ColumnPrimaryKey:
			pk = v
		case *ast.ColumnIdentity:
			identity = v
			identIdx = i
		}
	}

	autoGen := (pk != nil && pk.AutoIncrement) || identity != nil
	if !autoGen {
		return nil, false, nil
	}

	always := identity != nil && identity.Always
	isPK := pk != nil || hasMatchingTablePK

	switch dialect {
	case ast.DialectSQLite:
		if !isPK {
			return nil, false, errors.New(errors.ErrSQLDialectRewrite).
				Context("column \"" + col.Name + "\" auto-generates its value but is not the table's PRIMARY KEY; sqlite3 only supports AUTOINCREMENT on a single-column INTEGER PRIMARY KEY")
		}

		if col.Type == nil || col.Type.Name != "INTEGER" || len(col.Type.Args) != 0 {
			col.Type = &ast.TypeName{Name: "INTEGER"}
			notes = append(notes, "column \""+col.Name+"\": normalized type to INTEGER, required by sqlite3's AUTOINCREMENT")
		}

		if identity != nil {
			if always {
				notes = append(notes, "column \""+col.Name+"\": GENERATED ALWAYS AS IDENTITY has no sqlite3 equivalent; using AUTOINCREMENT, which (unlike ALWAYS) still permits an explicit inserted value")
			}

			col.Constraints = append(col.Constraints[:identIdx], col.Constraints[identIdx+1:]...)
		}

		if pk != nil {
			if !pk.AutoIncrement {
				pk.AutoIncrement = true

				notes = append(notes, "column \""+col.Name+"\": added AUTOINCREMENT, required by sqlite3 for a generated key")
			}

			return notes, false, nil
		}

		// isPK is true here with pk == nil, so hasMatchingTablePK must be
		// true: the PRIMARY KEY was written at table level. Promote it to
		// column level, the only place sqlite3 allows AUTOINCREMENT.
		col.Constraints = append(col.Constraints, &ast.ColumnPrimaryKey{AutoIncrement: true})

		return notes, true, nil

	case ast.DialectPostgreSQL:
		if pk != nil && pk.AutoIncrement {
			pk.AutoIncrement = false
			
			notes = append(notes, "column \""+col.Name+"\": rewrote AUTOINCREMENT as GENERATED BY DEFAULT AS IDENTITY, the modern PostgreSQL equivalent")
		}

		width := pgIntegerWidth(col.Type)
		if col.Type == nil || col.Type.Name != width || len(col.Type.Args) != 0 {
			col.Type = &ast.TypeName{Name: width}
		}

		if identity == nil {
			col.Constraints = append(col.Constraints, &ast.ColumnIdentity{Always: false})
		}

		return notes, false, nil
	}

	return notes, false, nil
}

// rewriteInsert translates sqlite3's INSERT OR <action> shorthand toward
// PostgreSQL, which has no such syntax — see the file comment for why the
// reverse direction, and the ON CONFLICT form both dialects already share,
// need no translation.
//
// Known limitations:
//   - "OR ABORT" is dropped outright: aborting the statement (and, absent
//     an active savepoint, the whole transaction) on a constraint violation
//     is already PostgreSQL's own default behavior for a plain INSERT, so
//     there is nothing left to add.
//   - "OR FAIL" and "OR ROLLBACK" have no PostgreSQL equivalent — a failed
//     statement inside a PostgreSQL transaction always poisons the whole
//     transaction, with no per-statement "keep what already succeeded"
//     option — and are reported as errors.ErrSQLDialectRewrite rather than
//     silently downgraded to plain-INSERT/OR-ABORT behavior.
//   - "OR REPLACE" needs an explicit column list (to know which columns to
//     overwrite) and, via uniqueKey, the target table's key columns (to
//     know what ON CONFLICT should target); either missing is reported as
//     an error rather than guessed at.
func rewriteInsert(s *ast.InsertStmt, dialect ast.Dialect, uniqueKey UniqueKeyLookup) ([]string, error) {
	if dialect != ast.DialectPostgreSQL || s.OrAction == "" {
		return nil, nil
	}

	action := s.OrAction
	s.OrAction = ""

	switch action {
	case "ABORT":
		return []string{"dropped OR ABORT (PostgreSQL's default behavior on a constraint violation already matches)"}, nil

	case "IGNORE":
		s.OnConflict = &ast.OnConflictClause{DoNothing: true}

		return []string{"rewrote INSERT OR IGNORE as INSERT ... ON CONFLICT DO NOTHING for PostgreSQL"}, nil

	case "REPLACE":
		if len(s.Columns) == 0 {
			return nil, errors.New(errors.ErrSQLDialectRewrite).
				Context("INSERT OR REPLACE into \"" + s.Table.Name + "\" has no explicit column list; PostgreSQL needs one to translate this to ON CONFLICT DO UPDATE")
		}

		if uniqueKey == nil {
			return nil, errors.New(errors.ErrSQLDialectRewrite).
				Context("INSERT OR REPLACE into \"" + s.Table.Name + "\" needs the table's key column(s) to translate to PostgreSQL, and none were available")
		}

		keyCols, err := uniqueKey(s.Table.Name)
		if err != nil {
			return nil, err
		}

		if len(keyCols) == 0 {
			return nil, errors.New(errors.ErrSQLDialectRewrite).
				Context("table \"" + s.Table.Name + "\" has no primary key or single-column unique index for ON CONFLICT to target")
		}

		isKeyCol := make(map[string]bool, len(keyCols))
		for _, c := range keyCols {
			isKeyCol[c] = true
		}

		var sets []*ast.SetClause

		for _, c := range s.Columns {
			if isKeyCol[c] {
				continue
			}

			sets = append(sets, &ast.SetClause{
				Columns: []string{c},
				// "excluded" is lower-case so it round-trips through
				// Format's PostgreSQL identifier quoting (every identifier
				// quoted, exact case preserved — see format.go's file
				// comment) and still matches ON CONFLICT's own pseudo-table
				// name, which PostgreSQL always folds to lower-case.
				Value: &ast.ColumnRef{Table: "excluded", Column: c},
			})
		}

		s.OnConflict = &ast.OnConflictClause{Target: keyCols, UpdateSet: sets, DoNothing: len(sets) == 0}

		return []string{"rewrote INSERT OR REPLACE into \"" + s.Table.Name + "\" as INSERT ... ON CONFLICT (...) DO UPDATE for PostgreSQL"}, nil

	default: // "FAIL", "ROLLBACK"
		return nil, errors.New(errors.ErrSQLDialectRewrite).
			Context("INSERT OR " + action + " has no PostgreSQL equivalent")
	}
}
