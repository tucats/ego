package sqlparse

import (
	"strings"
	"unicode"

	"github.com/tucats/ego/internal/sqlparse/ast"
)

// This file implements Format, the first of this package's higher-level
// AST-consuming helpers (see the file comment in sqlparse.go). It renders a
// parsed statement back to SQL text using a fixed, deterministic "pretty"
// style rather than reproducing the original source's exact whitespace:
//
//   - SQL keywords are written upper-case.
//   - Each major clause of a statement (SELECT/FROM/WHERE/GROUP BY/HAVING/
//     ORDER BY/LIMIT, and their counterparts on INSERT/UPDATE/DELETE/CREATE
//     TABLE/CREATE INDEX/CREATE VIEW) starts on its own line, at the
//     statement's base indentation.
//   - Explicit JOINs each get their own line, one per JOIN, so a long join
//     chain reads top to bottom instead of wrapping.
//   - A CREATE TABLE's column/constraint list, and a WITH clause's CTEs,
//     get one entry per line.
//   - A subquery — a parenthesized SELECT anywhere it appears: FROM,
//     EXISTS, IN, or a scalar subquery in an expression — has its body
//     indented one level deeper than its surrounding statement, with the
//     parentheses on their own lines.
//   - Everything else — a WHERE condition, a CASE expression, a function
//     call's argument list — is written on a single line. This first
//     version does not wrap long expressions across multiple lines; that
//     needs a width-aware approach quite different from the fixed-shape
//     rendering used here, and is a reasonable place for this package's
//     formatting to grow into later rather than something taken on now.
//
// Identifier quoting: PostgreSQL folds an unquoted identifier to lower
// case, so an identifier written with any upper-case letter must be
// double-quoted to round-trip with its original case intact. Format does
// this unconditionally, for every identifier, whenever the Sqlparse being
// formatted was parsed as PostgreSQL — trading a few more quotes than are
// strictly necessary (an all-lower-case name doesn't technically need
// quoting) for a simple, easy-to-verify rule: under PostgreSQL, every
// identifier this package emits is quoted, full stop. Under sqlite3, which
// never folds identifier case, Format only quotes an identifier when it
// must — when it isn't a plain run of letters/digits/underscores starting
// with a letter or underscore — since unquoted sqlite3 output reads more
// naturally and quoting sqlite3 output has no case-preservation purpose to
// justify it.
//
// Known limitations, honestly documented rather than silently handled:
//
//   - CreateTableStmt keeps column definitions and table-level constraints
//     in two separate slices (see ast/ddl.go) rather than one combined,
//     source-ordered list, because the parser building it doesn't track
//     their original interleaving. Format always emits all columns first,
//     then all constraints. This changes the source's exact layout but
//     never its meaning — SQL attaches no significance to that ordering.
//   - ColumnReferences.Table and TableForeignKey.RefTable (a FOREIGN KEY's
//     REFERENCES target) are stored as a single already-"schema.table"-
//     joined string rather than as separate parts (see parseReferencesClause
//     and parseQualifiedTableName in ddl.go/parser.go), so Format has no
//     way to quote the schema and table parts independently. It writes
//     this one field as-is, unquoted, in both dialects.
//   - A parenthesized join group in a FROM clause, e.g. "(a JOIN b ON ...)",
//     is parsed without recording that it was parenthesized in the source
//     (see the comment on that case in select.go's parseTableItem) — Format
//     has no parentheses to reproduce there and doesn't add its own, since
//     a plain join chain and a parenthesized one are the same tree.
//   - Function call names (FuncCall.Name) are written as-is, unquoted, even
//     under PostgreSQL — unlike table/column/alias names, quoting a
//     function name is visually unusual and rarely what a reader expects,
//     so Format accepts not preserving a user-defined function name's exact
//     case here.

// printer accumulates formatted output for one Format call. depth tracks
// the current indentation level, in units of one indentStep — see newline.
type printer struct {
	dialect ast.Dialect
	b       strings.Builder
	depth   int
}

// indentStep is the text written per indentation level. Four spaces is an
// arbitrary but common choice; there is nothing elsewhere in this file that
// depends on its exact width.
const indentStep = "    "

func (pr *printer) write(s string) {
	pr.b.WriteString(s)
}

// newline starts a new line at the printer's current indentation depth.
// Every clause-level line break produced by this file goes through here;
// expression formatting (format_expr.go) never calls it, which is exactly
// what keeps a WHERE condition or CASE expression on one line — see the
// file comment above.
func (pr *printer) newline() {
	pr.b.WriteByte('\n')
	pr.b.WriteString(strings.Repeat(indentStep, pr.depth))
}

func (pr *printer) indent() { pr.depth++ }
func (pr *printer) dedent() { pr.depth-- }

// ident writes name as a SQL identifier, double-quoting it when the
// dialect or the identifier's own shape requires it — see "Identifier
// quoting" in the file comment above for the rule.
func (pr *printer) ident(name string) {
	if pr.dialect == ast.DialectPostgreSQL || !isBareIdent(name) {
		pr.write(quoteIdent(name))

		return
	}

	pr.write(name)
}

// isBareIdent reports whether name can be written unquoted: non-empty,
// starting with a letter or underscore, and containing only letters,
// digits, and underscores thereafter. This deliberately does not check
// name against the SQL keyword list — this parser doesn't reserve keywords
// at all (see token.go's file comment), so neither does this quoting rule.
func isBareIdent(s string) bool {
	if s == "" {
		return false
	}

	for i, r := range s {
		switch {
		case i == 0 && (r == '_' || unicode.IsLetter(r)):
		case i > 0 && (r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)):
		default:
			return false
		}
	}

	return true
}

// quoteIdent double-quotes name, doubling any embedded double quote — the
// standard SQL escaping convention, and the same one lexer.go's
// scanQuotedIdent decodes on the way in.
func quoteIdent(name string) string {
	var b strings.Builder

	b.WriteByte('"')

	for _, r := range name {
		if r == '"' {
			b.WriteByte('"')
		}

		b.WriteRune(r)
	}

	b.WriteByte('"')

	return b.String()
}

// Format renders p's parsed statement back to SQL text; see the file
// comment above for the formatting rules it follows.
func (p Sqlparse) Format() string {
	pr := &printer{dialect: p.dialect}
	pr.statement(p.stmt)

	return pr.b.String()
}

// statement dispatches to the format function for s's concrete type. This
// is a plain Go type switch (see "How this parser works" in parser.go for
// the same technique used the other direction, to parse), not a method on
// ast.Node — keeping formatting logic here rather than as methods on the
// ast types themselves is what lets ast stay free of any particular
// rendering's opinions (see the ast package's own doc comment on staying
// "syntax only").
func (pr *printer) statement(s ast.Statement) {
	switch v := s.(type) {
	case *ast.SelectStmt:
		pr.selectStmt(v)
	case *ast.InsertStmt:
		pr.insertStmt(v)
	case *ast.UpdateStmt:
		pr.updateStmt(v)
	case *ast.DeleteStmt:
		pr.deleteStmt(v)
	case *ast.CreateTableStmt:
		pr.createTableStmt(v)
	case *ast.DropTableStmt:
		pr.dropTableStmt(v)
	case *ast.AlterTableStmt:
		pr.alterTableStmt(v)
	case *ast.CreateIndexStmt:
		pr.createIndexStmt(v)
	case *ast.DropIndexStmt:
		pr.dropIndexStmt(v)
	case *ast.CreateViewStmt:
		pr.createViewStmt(v)
	case *ast.DropViewStmt:
		pr.dropViewStmt(v)
	case *ast.BeginStmt:
		pr.beginStmt(v)
	case *ast.CommitStmt:
		pr.write("COMMIT")
	case *ast.RollbackStmt:
		pr.rollbackStmt(v)
	case *ast.SavepointStmt:
		pr.write("SAVEPOINT ")
		pr.ident(v.Name)
	case *ast.ReleaseStmt:
		pr.write("RELEASE ")
		pr.ident(v.Name)
	default:
		// Defensive fallback for a Statement implemented outside this
		// package (Statement's sealing means that can't actually happen
		// today — see ast/node.go — but the check is cheap and avoids a
		// nil dereference if that ever changes).
		if s != nil {
			pr.write(s.String())
		}
	}
}

func (pr *printer) beginStmt(v *ast.BeginStmt) {
	pr.write("BEGIN")

	if v.Mode != "" {
		pr.write(" ")
		pr.write(v.Mode)
	}

	if v.Name != "" {
		pr.write(" TRANSACTION ")
		pr.ident(v.Name)
	}
}

func (pr *printer) rollbackStmt(v *ast.RollbackStmt) {
	pr.write("ROLLBACK")

	if v.To != "" {
		pr.write(" TO ")
		pr.ident(v.To)
	}
}

// selectLike formats n, which is always a *ast.SelectStmt in practice (see
// e.g. parseSelectOrCompound in select.go — every AST field documented as
// holding "a *SelectStmt or *CompoundSelect" is, as of this parser, always
// the former, since only parseSelectBody ever constructs one and it always
// returns *ast.SelectStmt). It's factored out as its own small function,
// taking the general ast.Node type, so every call site that formats a
// nested SELECT — a subquery, a CTE body, an INSERT ... SELECT source, a
// CREATE VIEW's body — shares one place that does the type assertion and
// one fallback if it's ever wrong.
func (pr *printer) selectLike(n ast.Node) {
	if s, ok := n.(*ast.SelectStmt); ok {
		pr.selectStmt(s)

		return
	}

	if n != nil {
		pr.write(n.String())
	}
}
