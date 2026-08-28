package sqlparse

import (
	"strconv"
	"strings"

	"github.com/tucats/ego/internal/sqlparse/ast"
)

// This file implements the data-definition statements: CREATE/DROP/ALTER
// TABLE, CREATE/DROP INDEX, and CREATE/DROP VIEW. It's the largest file in
// the package mostly because CREATE TABLE's column and table constraint
// grammar (PRIMARY KEY, FOREIGN KEY, CHECK, DEFAULT, ...) is large, not
// because the parsing techniques here are new — everything below builds on
// the same recursive-descent shape, accept/expect helpers, and lookahead
// idioms explained in parser.go and expr.go.

// parseCreateStatement handles the part CREATE TABLE, CREATE INDEX, and
// CREATE VIEW all share — the keyword itself, and the optional "OR REPLACE"
// and "TEMP"/"TEMPORARY" modifiers, in that fixed order, before any of them
// actually diverge — and then dispatches to whichever of the three the next
// keyword names. orReplace and temp are parsed here even though neither is
// meaningful for CREATE INDEX, because at the point "CREATE" has been
// consumed there's no way yet to know which of the three statements this
// is. Consistent with this parser's syntax-only scope (see the package doc
// comment in parser.go), an author writing the nonsensical "CREATE OR
// REPLACE INDEX ..." is not rejected for it here — parseCreateIndexStatement
// below simply never receives orReplace, so it's parsed and then discarded.
func (p *parser) parseCreateStatement() (ast.Statement, error) {
	start := p.here()

	p.next() // "create"

	orReplace := false

	if p.isKeyword("or") {
		p.next()

		if err := p.expectKeyword("replace"); err != nil {
			return nil, err
		}

		orReplace = true
	}

	temp := p.acceptKeyword("temp") || p.acceptKeyword("temporary")

	switch {
	case p.isKeyword("unique") || p.isKeyword("index"):
		return p.parseCreateIndexStatement(start)
	case p.isKeyword("table"):
		return p.parseCreateTableStatement(start, temp)
	case p.isKeyword("view"):
		return p.parseCreateViewStatement(start, temp, orReplace)
	default:
		return nil, p.syntaxError("TABLE, VIEW, or INDEX")
	}
}

// parseIfNotExists parses the optional "IF NOT EXISTS" modifier accepted
// after CREATE TABLE/INDEX/VIEW.
func (p *parser) parseIfNotExists() (bool, error) {
	if !p.isKeyword("if") {
		return false, nil
	}

	p.next()

	if err := p.expectKeyword("not"); err != nil {
		return false, err
	}

	if err := p.expectKeyword("exists"); err != nil {
		return false, err
	}

	return true, nil
}

// parseIfExists parses the optional "IF EXISTS" modifier accepted after
// DROP TABLE/INDEX/VIEW.
func (p *parser) parseIfExists() (bool, error) {
	if !p.isKeyword("if") {
		return false, nil
	}

	p.next()

	if err := p.expectKeyword("exists"); err != nil {
		return false, err
	}

	return true, nil
}

// --- CREATE TABLE ---.

// parseCreateTableStatement parses everything after "CREATE [TEMP] TABLE":
// the name, then either the PostgreSQL "AS SELECT ..." shorthand (an early
// return — a CREATE TABLE ... AS SELECT has no column/constraint list of its
// own; the columns come from the SELECT) or the usual parenthesized
// column-and-constraint list, followed by sqlite3's optional
// "WITHOUT ROWID".
func (p *parser) parseCreateTableStatement(start ast.Position, temp bool) (ast.Statement, error) {
	p.next() // "table"

	ifNotExists, err := p.parseIfNotExists()
	if err != nil {
		return nil, err
	}

	table, err := p.parseTableRef()
	if err != nil {
		return nil, err
	}

	stmt := &ast.CreateTableStmt{
		Temp:        temp,
		IfNotExists: ifNotExists,
		Table:       table,
	}

	if p.isKeyword("as") {
		p.next()

		sel, err := p.parseSelectOrCompound()
		if err != nil {
			return nil, err
		}

		stmt.AsSelect = sel
		stmt.SetSpan(start, p.here())

		return stmt, nil
	}

	if err := p.expectPunct("("); err != nil {
		return nil, err
	}

	for {
		if p.tableConstraintFollows() {
			c, err := p.parseTableConstraint()
			if err != nil {
				return nil, err
			}

			stmt.Constraints = append(stmt.Constraints, c)
		} else {
			col, err := p.parseColumnDef()
			if err != nil {
				return nil, err
			}

			stmt.Columns = append(stmt.Columns, col)
		}

		if !p.acceptPunct(",") {
			break
		}
	}

	if err := p.expectPunct(")"); err != nil {
		return nil, err
	}

	if p.isKeyword("without") {
		p.next()

		if err := p.expectKeyword("rowid"); err != nil {
			return nil, err
		}

		stmt.WithoutRowID = true
	}

	stmt.SetSpan(start, p.here())

	return stmt, nil
}

// tableConstraintFollows reports whether the current token starts a
// table-level constraint (as opposed to a column definition). Both are
// legal, comma-separated, in any order, inside a CREATE TABLE's parens —
// "CREATE TABLE t (id INT, name TEXT, PRIMARY KEY (id))" has two column
// definitions followed by one table constraint — and this lookahead is what
// parseCreateTableStatement's loop uses to tell, per comma-separated item,
// which of the two it's about to parse: a column definition always starts
// with the column's own name (an arbitrary identifier), while a table
// constraint always starts with one of these five fixed keywords, so
// checking for the keywords is sufficient to disambiguate.
func (p *parser) tableConstraintFollows() bool {
	return p.isKeyword("constraint") || p.isKeyword("primary") ||
		p.isKeyword("unique") || p.isKeyword("foreign") || p.isKeyword("check")
}

// columnConstraintFollows reports whether the current token starts a
// column-level constraint (PRIMARY KEY, NOT NULL, UNIQUE, CHECK, DEFAULT,
// REFERENCES, COLLATE, or a GENERATED/AS computed-column clause). It serves
// two purposes in parseColumnDef below: deciding when the column's optional
// type name has ended (a type name is just a run of bare words — see
// parseTypeName — so parsing stops as soon as one of these keywords, or a
// non-identifier token, appears), and then driving the loop that parses
// each constraint in turn.
func (p *parser) columnConstraintFollows() bool {
	return p.isKeyword("constraint") || p.isKeyword("primary") || p.isKeyword("not") ||
		p.isKeyword("unique") || p.isKeyword("check") || p.isKeyword("default") ||
		p.isKeyword("references") || p.isKeyword("collate") || p.isKeyword("generated") ||
		p.isKeyword("as")
}

// parseColumnDef parses one column definition: its name, an optional type
// name (sqlite3 permits a column with no declared type at all — "CREATE
// TABLE t (a)" is legal sqlite3 — which is why the type is skipped
// entirely when a constraint keyword or non-identifier follows the name
// directly), and zero or more constraints.
func (p *parser) parseColumnDef() (*ast.ColumnDef, error) {
	start := p.here()

	name, err := p.expectIdent()
	if err != nil {
		return nil, err
	}

	col := &ast.ColumnDef{Name: name}

	if p.cur().kind == tokIdent && !p.columnConstraintFollows() {
		typ, err := p.parseTypeName()
		if err != nil {
			return nil, err
		}

		col.Type = typ

		// PostgreSQL's SERIAL/BIGSERIAL/SMALLSERIAL are pseudo-types, sugar
		// for "the realName integer type, plus a GENERATED BY DEFAULT AS
		// IDENTITY constraint" (see serialTypeAliases and ColumnIdentity's
		// doc comment). Desugaring here, at parse time, means every later
		// consumer of a ColumnDef — Rewrite, Format, anything a caller
		// writes against this AST — only ever has to deal with the one
		// canonical ColumnIdentity representation, regardless of which of
		// the three surface spellings (SERIAL, AUTOINCREMENT, or GENERATED
		// ... AS IDENTITY) the client actually wrote.
		if realName, ok := serialTypeAliases[typ.Name]; ok && len(typ.Args) == 0 {
			col.Type = &ast.TypeName{Name: realName}
			col.Type.SetSpan(typ.Pos(), typ.End())

			ident := &ast.ColumnIdentity{Always: false}
			ident.SetSpan(typ.Pos(), typ.End())

			col.Constraints = append(col.Constraints, ident)
		}
	}

	for p.columnConstraintFollows() {
		c, err := p.parseColumnConstraint()
		if err != nil {
			return nil, err
		}

		col.Constraints = append(col.Constraints, c)
	}

	col.SetSpan(start, p.here())

	return col, nil
}

// skipParenGroup consumes a balanced "( ... )" group without interpreting
// what's inside it, for constructs this syntax-only parser has no reason to
// model in detail — currently just the optional sequence-option list after
// "GENERATED ... AS IDENTITY" (e.g. "(START WITH 1 INCREMENT BY 1)"). The
// opening "(" must be the current token.
func (p *parser) skipParenGroup() error {
	if err := p.expectPunct("("); err != nil {
		return err
	}

	depth := 1

	for depth > 0 {
		switch {
		case p.atEnd():
			return p.syntaxError(")")
		case p.cur().isOp("("):
			depth++
		case p.cur().isOp(")"):
			depth--
		}

		p.next()
	}

	return nil
}

// serialTypeAliases maps each of PostgreSQL's SERIAL-family pseudo-types
// (parsed generically, like any other type name, by parseTypeName) to the
// real integer type it desugars to — see the ColumnIdentity substitution in
// parseColumnDef above.
var serialTypeAliases = map[string]string{
	"SMALLSERIAL": "SMALLINT",
	"SERIAL":      "INTEGER",
	"BIGSERIAL":   "BIGINT",
}

// parseTypeName parses a column or CAST type name: one or more bare words
// (DOUBLE PRECISION, UNSIGNED BIG INT, ...) followed by an optional
// one-or-two-argument size/precision list.
func (p *parser) parseTypeName() (*ast.TypeName, error) {
	start := p.here()

	var words []string

	for p.cur().kind == tokIdent && !p.columnConstraintFollows() &&
		!p.cur().isOp("(") && !p.cur().isOp(",") && !p.cur().isOp(")") {
		w, err := p.expectIdent()
		if err != nil {
			return nil, err
		}

		words = append(words, strings.ToUpper(w))
	}

	if len(words) == 0 {
		return nil, p.syntaxError("a type name")
	}

	var args []int

	if p.cur().isOp("(") {
		p.next()

		n, err := p.expectIntLiteral()
		if err != nil {
			return nil, err
		}

		args = append(args, n)

		if p.acceptPunct(",") {
			n2, err := p.expectIntLiteral()
			if err != nil {
				return nil, err
			}

			args = append(args, n2)
		}

		if err := p.expectPunct(")"); err != nil {
			return nil, err
		}
	}

	typ := &ast.TypeName{Name: strings.Join(words, " "), Args: args}
	typ.SetSpan(start, p.here())

	return typ, nil
}

// expectIntLiteral consumes a numeric token and parses it as an unsigned
// int, for a type's size/precision arguments (VARCHAR(255), NUMERIC(10,2)).
func (p *parser) expectIntLiteral() (int, error) {
	t := p.cur()
	if t.kind != tokNumber {
		return 0, p.syntaxError("a number")
	}

	p.next()

	n, err := strconv.Atoi(t.text)
	if err != nil {
		return 0, p.syntaxError("a number")
	}

	return n, nil
}

// parseColumnConstraint parses one column-level constraint. columnConstraintFollows
// above has already confirmed one of the eight keywords this switch matches
// on is present, so the "default" case (reached only if that guarantee is
// somehow violated) is unreachable in practice but kept as a defensive
// syntax error rather than a panic. Every branch shares the same optional
// leading "CONSTRAINT name" — parsed once, before the switch, since it can
// precede any of the eight kinds.
func (p *parser) parseColumnConstraint() (ast.Node, error) {
	start := p.here()

	name := ""

	if p.acceptKeyword("constraint") {
		n, err := p.expectIdent()
		if err != nil {
			return nil, err
		}

		name = n
	}

	switch {
	case p.isKeyword("primary"):
		p.next()

		if err := p.expectKeyword("key"); err != nil {
			return nil, err
		}

		desc := false

		switch {
		case p.acceptKeyword("asc"):
		case p.acceptKeyword("desc"):
			desc = true
		}

		auto := p.acceptKeyword("autoincrement")

		conflict, err := p.parseOptionalConflictClause()
		if err != nil {
			return nil, err
		}

		n := &ast.ColumnPrimaryKey{Name: name, Desc: desc, AutoIncrement: auto, Conflict: conflict}
		n.SetSpan(start, p.here())

		return n, nil

	case p.isKeyword("not"):
		p.next()

		if err := p.expectKeyword("null"); err != nil {
			return nil, err
		}

		conflict, err := p.parseOptionalConflictClause()
		if err != nil {
			return nil, err
		}

		n := &ast.ColumnNotNull{Name: name, Conflict: conflict}
		n.SetSpan(start, p.here())

		return n, nil

	case p.isKeyword("unique"):
		p.next()

		conflict, err := p.parseOptionalConflictClause()
		if err != nil {
			return nil, err
		}

		n := &ast.ColumnUnique{Name: name, Conflict: conflict}
		n.SetSpan(start, p.here())

		return n, nil

	case p.isKeyword("check"):
		p.next()

		if err := p.expectPunct("("); err != nil {
			return nil, err
		}

		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		end := p.here()

		if err := p.expectPunct(")"); err != nil {
			return nil, err
		}

		n := &ast.ColumnCheck{Name: name, Expr: expr}
		n.SetSpan(start, end)

		return n, nil

	case p.isKeyword("default"):
		p.next()

		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		n := &ast.ColumnDefault{Value: val}
		n.SetSpan(start, val.End())

		return n, nil

	case p.isKeyword("references"):
		p.next()

		return p.parseReferencesClause(start, name)

	case p.isKeyword("collate"):
		p.next()

		coll, err := p.expectIdent()
		if err != nil {
			return nil, err
		}

		n := &ast.ColumnCollate{Collation: coll}
		n.SetSpan(start, p.here())

		return n, nil

	case p.isKeyword("generated") || p.isKeyword("as"):
		// "always" defaults to true for the bare "AS (expr)" computed-column
		// spelling (which has no ALWAYS/BY DEFAULT choice at all) and for
		// "GENERATED ALWAYS AS ..."; it's only set false by an explicit
		// "GENERATED BY DEFAULT AS IDENTITY" — PostgreSQL's generated-key
		// form, see ColumnIdentity's doc comment. A computed column can only
		// ever be ALWAYS (there is no "GENERATED BY DEFAULT AS (expr)" in
		// either dialect), so a client that writes that nonsensical
		// combination just silently gets an ALWAYS computed column back —
		// consistent with this parser's syntax-only scope (see the package
		// doc comment in parser.go).
		always := true

		if p.acceptKeyword("generated") {
			switch {
			case p.acceptKeyword("always"):
				always = true
			case p.acceptKeyword("by"):
				if err := p.expectKeyword("default"); err != nil {
					return nil, err
				}

				always = false
			default:
				return nil, p.syntaxError("ALWAYS or BY DEFAULT")
			}
		}

		if err := p.expectKeyword("as"); err != nil {
			return nil, err
		}

		// "AS IDENTITY [(seq_options)]" (PostgreSQL's generated-key
		// constraint) vs "AS (expr) [STORED|VIRTUAL]" (a computed column) —
		// the keyword right after AS is enough to tell them apart without
		// any further lookahead.
		if p.acceptKeyword("identity") {
			if p.cur().isOp("(") {
				if err := p.skipParenGroup(); err != nil {
					return nil, err
				}
			}

			n := &ast.ColumnIdentity{Always: always}
			n.SetSpan(start, p.here())

			return n, nil
		}

		if err := p.expectPunct("("); err != nil {
			return nil, err
		}

		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		if err := p.expectPunct(")"); err != nil {
			return nil, err
		}

		stored := false

		switch {
		case p.acceptKeyword("stored"):
			stored = true
		case p.acceptKeyword("virtual"):
			stored = false
		}

		n := &ast.ColumnGenerated{Expr: expr, Stored: stored}
		n.SetSpan(start, p.here())

		return n, nil

	default:
		return nil, p.syntaxError("a column constraint")
	}
}

// parseOptionalConflictClause parses sqlite3's optional "ON CONFLICT action"
// suffix on a PRIMARY KEY, NOT NULL, or UNIQUE constraint (not to be
// confused with INSERT's "ON CONFLICT ... DO ..." clause in common.go,
// which is a different, statement-level construct that happens to share the
// same two leading keywords). isKeywordAt(1, "conflict") looks one token
// past "on" before committing to consume anything, since a bare "ON" here
// would otherwise be indistinguishable at a glance from the start of some
// other construct — the same lookahead-before-committing idiom used
// throughout this package; see "How this parser works" in parser.go.
func (p *parser) parseOptionalConflictClause() (string, error) {
	if !(p.isKeyword("on") && p.isKeywordAt(1, "conflict")) {
		return "", nil
	}

	p.next()
	p.next()

	switch {
	case p.acceptKeyword("rollback"):
		return "ROLLBACK", nil
	case p.acceptKeyword("abort"):
		return "ABORT", nil
	case p.acceptKeyword("fail"):
		return "FAIL", nil
	case p.acceptKeyword("ignore"):
		return "IGNORE", nil
	case p.acceptKeyword("replace"):
		return "REPLACE", nil
	default:
		return "", p.syntaxError("ROLLBACK, ABORT, FAIL, IGNORE, or REPLACE")
	}
}

// parseReferencesClause parses a column-level "REFERENCES table [(cols...)]
// [ON DELETE ...] [ON UPDATE ...] [[NOT] DEFERRABLE [INITIALLY ...]]",
// following an already-consumed REFERENCES keyword. name is the constraint
// name parsed by the caller (parseColumnConstraint), if any, and is just
// threaded through onto the resulting node.
func (p *parser) parseReferencesClause(start ast.Position, name string) (*ast.ColumnReferences, error) {
	table, err := p.parseQualifiedTableName()
	if err != nil {
		return nil, err
	}

	var cols []string

	if p.cur().isOp("(") {
		cols, err = p.parseNameList()
		if err != nil {
			return nil, err
		}
	}

	onDelete, onUpdate, err := p.parseReferentialActions()
	if err != nil {
		return nil, err
	}

	deferrable, initially := p.parseDeferrableClause()

	n := &ast.ColumnReferences{
		Name: name, Table: table, Columns: cols,
		OnDelete: onDelete, OnUpdate: onUpdate,
		Deferrable: deferrable, Initially: initially,
	}
	n.SetSpan(start, p.here())

	return n, nil
}

// parseReferentialActions parses zero or more "ON DELETE action" / "ON
// UPDATE action" clauses in either order, returning whichever were seen.
func (p *parser) parseReferentialActions() (onDelete, onUpdate string, err error) {
	for p.isKeyword("on") {
		p.next()

		switch {
		case p.acceptKeyword("delete"):
			action, err := p.parseReferentialAction()
			if err != nil {
				return "", "", err
			}

			onDelete = action
		case p.acceptKeyword("update"):
			action, err := p.parseReferentialAction()
			if err != nil {
				return "", "", err
			}

			onUpdate = action
		default:
			return "", "", p.syntaxError("DELETE or UPDATE")
		}
	}

	return onDelete, onUpdate, nil
}

// parseReferentialAction parses the action word after "ON DELETE"/"ON
// UPDATE": CASCADE, RESTRICT, SET NULL, SET DEFAULT, or NO ACTION.
func (p *parser) parseReferentialAction() (string, error) {
	switch {
	case p.acceptKeyword("cascade"):
		return "CASCADE", nil
	case p.acceptKeyword("restrict"):
		return "RESTRICT", nil
	case p.isKeyword("set"):
		p.next()

		switch {
		case p.acceptKeyword("null"):
			return "SET NULL", nil
		case p.acceptKeyword("default"):
			return "SET DEFAULT", nil
		default:
			return "", p.syntaxError("NULL or DEFAULT")
		}
	case p.isKeyword("no"):
		p.next()

		if err := p.expectKeyword("action"); err != nil {
			return "", err
		}

		return "NO ACTION", nil
	default:
		return "", p.syntaxError("CASCADE, RESTRICT, SET NULL, SET DEFAULT, or NO ACTION")
	}
}

// parseDeferrableClause parses an optional trailing "[NOT] DEFERRABLE
// [INITIALLY DEFERRED|IMMEDIATE]" on a foreign key constraint.
func (p *parser) parseDeferrableClause() (deferrable, initially string) {
	switch {
	case p.isKeyword("not") && p.isKeywordAt(1, "deferrable"):
		p.next()
		p.next()

		deferrable = "NOT DEFERRABLE"
	case p.acceptKeyword("deferrable"):
		deferrable = "DEFERRABLE"
	default:
		return "", ""
	}

	if p.acceptKeyword("initially") {
		switch {
		case p.acceptKeyword("deferred"):
			initially = "DEFERRED"
		case p.acceptKeyword("immediate"):
			initially = "IMMEDIATE"
		}
	}

	return deferrable, initially
}

// parseTableConstraint parses one table-level constraint (PRIMARY KEY,
// UNIQUE, FOREIGN KEY, or CHECK) — the counterpart to parseColumnConstraint
// above, called instead of parseColumnDef when tableConstraintFollows
// reports the current comma-separated item in a CREATE TABLE's column list
// is a constraint rather than a column. Table-level PRIMARY KEY/UNIQUE take
// an explicit column list (they apply across one or more named columns,
// unlike the column-level forms which implicitly apply to the column
// they're attached to), and table-level FOREIGN KEY additionally names its
// own local column(s) before the REFERENCES target.
func (p *parser) parseTableConstraint() (ast.Node, error) {
	start := p.here()

	name := ""

	if p.acceptKeyword("constraint") {
		n, err := p.expectIdent()
		if err != nil {
			return nil, err
		}

		name = n
	}

	switch {
	case p.isKeyword("primary"):
		p.next()

		if err := p.expectKeyword("key"); err != nil {
			return nil, err
		}

		cols, err := p.parseIndexColumnList()
		if err != nil {
			return nil, err
		}

		conflict, err := p.parseOptionalConflictClause()
		if err != nil {
			return nil, err
		}

		n := &ast.TablePrimaryKey{Name: name, Columns: cols, Conflict: conflict}
		n.SetSpan(start, p.here())

		return n, nil

	case p.isKeyword("unique"):
		p.next()

		cols, err := p.parseIndexColumnList()
		if err != nil {
			return nil, err
		}

		conflict, err := p.parseOptionalConflictClause()
		if err != nil {
			return nil, err
		}

		n := &ast.TableUnique{Name: name, Columns: cols, Conflict: conflict}
		n.SetSpan(start, p.here())

		return n, nil

	case p.isKeyword("foreign"):
		p.next()

		if err := p.expectKeyword("key"); err != nil {
			return nil, err
		}

		cols, err := p.parseNameList()
		if err != nil {
			return nil, err
		}

		if err := p.expectKeyword("references"); err != nil {
			return nil, err
		}

		refTable, err := p.parseQualifiedTableName()
		if err != nil {
			return nil, err
		}

		var refCols []string

		if p.cur().isOp("(") {
			refCols, err = p.parseNameList()
			if err != nil {
				return nil, err
			}
		}

		onDelete, onUpdate, err := p.parseReferentialActions()
		if err != nil {
			return nil, err
		}

		deferrable, initially := p.parseDeferrableClause()

		n := &ast.TableForeignKey{
			Name: name, Columns: cols, RefTable: refTable, RefColumns: refCols,
			OnDelete: onDelete, OnUpdate: onUpdate,
			Deferrable: deferrable, Initially: initially,
		}
		n.SetSpan(start, p.here())

		return n, nil

	case p.isKeyword("check"):
		p.next()

		if err := p.expectPunct("("); err != nil {
			return nil, err
		}

		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		end := p.here()

		if err := p.expectPunct(")"); err != nil {
			return nil, err
		}

		n := &ast.TableCheck{Name: name, Expr: expr}
		n.SetSpan(start, end)

		return n, nil

	default:
		return nil, p.syntaxError("PRIMARY KEY, UNIQUE, FOREIGN KEY, or CHECK")
	}
}

// --- DROP TABLE / DROP INDEX / DROP VIEW ---.

// parseDropStatement dispatches DROP to whichever of TABLE/INDEX/VIEW the
// next keyword names, the same one-keyword-of-lookahead approach
// parser.parseStatement uses for the top-level statement dispatch.
func (p *parser) parseDropStatement() (ast.Statement, error) {
	start := p.here()

	p.next() // "drop"

	switch {
	case p.isKeyword("table"):
		return p.parseDropTableStatement(start)
	case p.isKeyword("index"):
		return p.parseDropIndexStatement(start)
	case p.isKeyword("view"):
		return p.parseDropViewStatement(start)
	default:
		return nil, p.syntaxError("TABLE, INDEX, or VIEW")
	}
}

// parseDropTableStatement parses "DROP TABLE [IF EXISTS] table [CASCADE |
// RESTRICT]", called with the "DROP" keyword already consumed by
// parseDropStatement and start marking where it began.
func (p *parser) parseDropTableStatement(start ast.Position) (ast.Statement, error) {
	p.next() // "table"

	ifExists, err := p.parseIfExists()
	if err != nil {
		return nil, err
	}

	table, err := p.parseTableRef()
	if err != nil {
		return nil, err
	}

	stmt := &ast.DropTableStmt{IfExists: ifExists, Table: table}

	switch {
	case p.acceptKeyword("cascade"):
		stmt.Cascade = true
	case p.acceptKeyword("restrict"):
		stmt.Restrict = true
	}

	stmt.SetSpan(start, p.here())

	return stmt, nil
}

// parseDropIndexStatement parses "DROP INDEX [IF EXISTS] [schema.]name".
func (p *parser) parseDropIndexStatement(start ast.Position) (ast.Statement, error) {
	p.next() // "index"

	ifExists, err := p.parseIfExists()
	if err != nil {
		return nil, err
	}

	schema, name, err := p.parseSchemaQualifiedName()
	if err != nil {
		return nil, err
	}

	stmt := &ast.DropIndexStmt{IfExists: ifExists, Schema: schema, Name: name}
	stmt.SetSpan(start, p.here())

	return stmt, nil
}

// parseDropViewStatement parses "DROP VIEW [IF EXISTS] [schema.]name
// [CASCADE | RESTRICT]".
func (p *parser) parseDropViewStatement(start ast.Position) (ast.Statement, error) {
	p.next() // "view"

	ifExists, err := p.parseIfExists()
	if err != nil {
		return nil, err
	}

	schema, name, err := p.parseSchemaQualifiedName()
	if err != nil {
		return nil, err
	}

	stmt := &ast.DropViewStmt{IfExists: ifExists, Schema: schema, Name: name}

	switch {
	case p.acceptKeyword("cascade"):
		stmt.Cascade = true
	case p.acceptKeyword("restrict"):
		stmt.Restrict = true
	}

	stmt.SetSpan(start, p.here())

	return stmt, nil
}

// --- ALTER TABLE ---.

// parseAlterTableStatement parses "ALTER TABLE table action", where action
// is one of ADD/DROP/RENAME — see parseAlterTableAction below.
func (p *parser) parseAlterTableStatement() (ast.Statement, error) {
	start := p.here()

	p.next() // "alter"

	if err := p.expectKeyword("table"); err != nil {
		return nil, err
	}

	table, err := p.parseTableRef()
	if err != nil {
		return nil, err
	}

	action, err := p.parseAlterTableAction()
	if err != nil {
		return nil, err
	}

	stmt := &ast.AlterTableStmt{Table: table, Action: action}
	stmt.SetSpan(start, p.here())

	return stmt, nil
}

// parseAlterTableAction parses the part of ALTER TABLE after the table
// name: ADD [COLUMN], DROP [COLUMN], RENAME TO, or RENAME [COLUMN] ... TO
// .... Note the COLUMN keyword is always optional here (p.acceptKeyword,
// not p.expectKeyword) — both "ADD a INT" and "ADD COLUMN a INT" are legal.
// RENAME needs one extra token of lookahead (via acceptKeyword("to")) to
// tell "RENAME TO newname" (renaming the table itself) apart from "RENAME
// [COLUMN] old TO new" (renaming a column), since both start with RENAME
// and neither has another distinguishing keyword before the ambiguous part.
func (p *parser) parseAlterTableAction() (ast.Node, error) {
	start := p.here()

	switch {
	case p.isKeyword("add"):
		p.next()
		p.acceptKeyword("column")

		col, err := p.parseColumnDef()
		if err != nil {
			return nil, err
		}

		n := &ast.AddColumn{Column: col}
		n.SetSpan(start, p.here())

		return n, nil

	case p.isKeyword("drop"):
		p.next()
		p.acceptKeyword("column")

		name, err := p.expectIdent()
		if err != nil {
			return nil, err
		}

		n := &ast.DropColumn{Name: name}
		n.SetSpan(start, p.here())

		return n, nil

	case p.isKeyword("rename"):
		p.next()

		if p.acceptKeyword("to") {
			name, err := p.expectIdent()
			if err != nil {
				return nil, err
			}

			n := &ast.RenameTable{To: name}
			n.SetSpan(start, p.here())

			return n, nil
		}

		p.acceptKeyword("column")

		from, err := p.expectIdent()
		if err != nil {
			return nil, err
		}

		if err := p.expectKeyword("to"); err != nil {
			return nil, err
		}

		to, err := p.expectIdent()
		if err != nil {
			return nil, err
		}

		n := &ast.RenameColumn{From: from, To: to}
		n.SetSpan(start, p.here())

		return n, nil

	default:
		return nil, p.syntaxError("ADD, DROP, or RENAME")
	}
}

// --- CREATE INDEX / DROP INDEX ---.

// parseCreateIndexStatement parses "CREATE [UNIQUE] INDEX [IF NOT EXISTS]
// name ON [schema.]table (columns...) [WHERE where]", called with "CREATE"
// (and any OR REPLACE/TEMP modifiers — see parseCreateStatement above)
// already consumed, so it starts by looking for the optional UNIQUE itself.
func (p *parser) parseCreateIndexStatement(start ast.Position) (ast.Statement, error) {
	unique := p.acceptKeyword("unique")

	if err := p.expectKeyword("index"); err != nil {
		return nil, err
	}

	ifNotExists, err := p.parseIfNotExists()
	if err != nil {
		return nil, err
	}

	name, err := p.expectIdent()
	if err != nil {
		return nil, err
	}

	if err := p.expectKeyword("on"); err != nil {
		return nil, err
	}

	tableSchema, table, err := p.parseSchemaQualifiedName()
	if err != nil {
		return nil, err
	}

	cols, err := p.parseIndexColumnList()
	if err != nil {
		return nil, err
	}

	stmt := &ast.CreateIndexStmt{
		Unique: unique, IfNotExists: ifNotExists, Name: name, Schema: tableSchema, Table: table, Columns: cols,
	}

	if p.acceptKeyword("where") {
		w, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		stmt.Where = w
	}

	stmt.SetSpan(start, p.here())

	return stmt, nil
}

// --- CREATE VIEW / DROP VIEW ---.

// parseCreateViewStatement parses "CREATE [OR REPLACE] [TEMP] VIEW
// [IF NOT EXISTS] [schema.]name [(columns...)] AS select", called with
// "VIEW" still to be consumed and temp/orReplace already decided by
// parseCreateStatement.
func (p *parser) parseCreateViewStatement(start ast.Position, temp, orReplace bool) (ast.Statement, error) {
	p.next() // "view"

	ifNotExists, err := p.parseIfNotExists()
	if err != nil {
		return nil, err
	}

	schema, name, err := p.parseSchemaQualifiedName()
	if err != nil {
		return nil, err
	}

	stmt := &ast.CreateViewStmt{OrReplace: orReplace, Temp: temp, IfNotExists: ifNotExists, Schema: schema, Name: name}

	if p.cur().isOp("(") {
		cols, err := p.parseNameList()
		if err != nil {
			return nil, err
		}

		stmt.Columns = cols
	}

	if err := p.expectKeyword("as"); err != nil {
		return nil, err
	}

	sel, err := p.parseSelectOrCompound()
	if err != nil {
		return nil, err
	}

	stmt.Select = sel
	stmt.SetSpan(start, p.here())

	return stmt, nil
}
